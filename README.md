# slo-builder [![Documentation](https://godoc.org/github.com/gocardless/slo-builder/pkg/templates?status.svg)](http://godoc.org/github.com/gocardless/slo-builder/pkg/templates)

This repo provides a framework that developers can use to specify system SLOs
without requiring in-depth Prometheus knowledge.

## Why?

SLOs are often formulated in business terms first, then translated into
monitoring system rules. Good SLOs should be formed as a ratio of good events to
total events, and come with an associated error budget- the margin of error
you'd expect to consume in normal operation.

By forcing a homogenous format for every SLO, it becomes possible to apply
generically useful rules to all different types of SLO. This is even more
important when the implementation of such rules are so tricky, and the required
learning to produce them so large.

## Quick start

1. Define a new SLO using one of the supported templates.

```yaml
---
definitions:
  - template: ErrorRateSLO
    definition:
      name: APIErrorRate
      budget: 0.001
      errors: |
        rate(http_request_duration_seconds_count{status=~"5.."}[%s])
      total: |
        rate(http_request_duration_seconds_count[%s])
```

The above example, is using the `ErrorRateSLO` template, that requires the
following:

- `name` unique name for SLO definition.
- `budget` error budget 0.1% in this case.
- `errors` parameterize rate of error requests.
- `total` parameterize rate of requests.

> `%s` is replaced with multiple alert windows

2. Generate Prometheus rules

```bash
$ slo-builder build examples/*-slo.yaml > examples/rules.yaml
```

> You can check more examples inside [examples](./examples) folder and the generated
Prometheuls rules [examples/rules.yaml](./examples/rules.yaml)

## Steps to an SLO

You start with a system, often with a number of SLIs. You then:

1. Formulate SLOs in business terms
2. Implement the SLOs in the monitoring system (Prometheus)
3. Write multi-window alerts for burning error budgets

System components will need different categories of SLO: an SLO for HTTP
requests will be structured differently than a batch processing system, for
example. This framework offers a collection of predefined templates that map to
different types of system, and can help someone unfamiliar with SLOs quickly
produce rules that Just Work.

When forumating SLOs in business terms (1), you can use these predefined
templates to help inform your selections. This framework can then produce the
rules (2) that generate a common input to multi-windowed alerts (3), which are
included at the end of this SLO pipeline.

### `baseSLO`

Every SLO has some common behaviour, as represented by the `baseSLO` type. An
example core SLO definition would be:

```go
baseSLO{
  Name: "MarkPaymentsAsPaidMeetsDeadline",
  ErrorBudget: 0.1,
}
```

In rule form, we produce a `job:slo_definition:none` rule which tracks the
parameters of the base SLO. This writes a time-series that can be inspected for
how the SLO definition changed with time.

We also produce a `job:slo_error_budget:ratio` which will be used at the end of
the SLO pipeline to apply alerting rules. Each of these rules has the `name`
label that is assumed to be unique to each SLO, allowing Prometheus to join
series on the `name` label.

```
job:slo_definition:none{name="MarkPaymentsAsPaidMeetsDeadline",budget="0.1",template="BatchProcessingSLO"} 1.0
job:slo_error_budget:ratio{name="MarkPaymentsAsPaidMeetsDeadline"}
```

### `ErrorRateSLO`

Template to construct SLOs based on error rate.


### `LatencySLO`

Template to construct SLOs based on latency.

### `BatchProcessingSLO`

We'll use an example of a process that transitions many payments into a paid
state as a batch process for which we want to apply an SLO.

```go
MarkPaymentsAsPaidMeetsDeadline = BatchProcessingSLO{
  baseSLO:  baseSLO{"MarkPaymentsAsPaidMeetsDeadline", 0.1},
  Deadline: time.Duration(2) * time.Hour,
  Volume: `
  1.5 * max_over_time(
    (
      sum by (namespace, release) (
        increase(paysvc_mark_payments_as_paid_marked_as_paid_total[8h])
      )
    )[60d:1h]
  )`,
  Throughput: `
  sum by (namespace, release) (
    rate(paysvc_mark_payments_as_paid_marked_as_paid_total[1m])
  ) > 0`,
}
```

Users provide a time by which an entire batch must complete, along with an
estimation of max volume and a current measurement of throughput. This is enough
information to infer a target throughput (knowing how fast items need processing
to hit the deadline) which can be used to score each minute of activity from the
job.

In total, we produce three rules for this specific SLO:

```
job:slo_batch_volume:max{name="MarkPaymentsAsPaidMeetsDeadline",namespace="production",release="paysvc-live"}	2058877.53186143
job:slo_batch_throughput_target:max{name="MarkPaymentsAsPaidMeetsDeadline",namespace="production",release="paysvc-live"} 285.95521275853196
job:slo_batch_throughput:interval{name="MarkPaymentsAsPaidMeetsDeadline",namespace="production",release="paysvc-live"} 369.7091257289802
```

These rules are then consumed by a generic batch processing rule that translates
into `job:slo_error:ratio<I>` rules for each common alert window. The most
important rule is the generation of an error 'score' for the batch process,
which looks like:

```yaml
- record: job:slo_batch_error:interval
  expr: |
    1.0 - clamp_max(
      job:slo_batch_throughput:interval / job:slo_batch_throughput_target:max,
      1.0
    )
```

In this context, `job:slo_batch_error:interval` is the error score for each
interval of the throughput given in the SLO specification. For
MarkPaymentsAsPaid, with the target throughput of 285 payments/s calculated over
a 1m window, you'd have the following scores:

| Throughput | `job:slo_batch_throughput_target:max` | `job:slo_batch_error:interval` |
| --- | --- | --- |
| 0 | 285 | 100% |
| 100 | 285 | 65% |
| 285 | 285 | 0% |
| 300 | 285 | 0% |

This means you burn your error budget when the batch job performs below the
target throughput, and the rate at which you burn it is dependent on how
significantly you fail to meet it. It's also important to note that minutes
where the throughput greatly exceeds the target don't 'recoup' error budget-
this is an implementation decision, and might be the wrong choice.

## Alerting

Every SLO template conforms to our definition of an SLO, which is something that
has a name, associated error budget and a constantly refreshed error ratio. In
Prometheus terms, that means your SLOs will eventually produce the following
time series:

- `job:slo_error:ratio1m`
- `job:slo_error:ratio5m`
- `job:slo_error:ratio30m`
- `job:slo_error:ratio1h`
- `job:slo_error:ratio2h`
- `job:slo_error:ratio6h`
- `job:slo_error:ratio1d`
- `job:slo_error:ratio7d`

As we get these series for every SLO, we can write generic alerting rules that
work across any SLO. It happens that building useful alerts on SLO measurements
is more complex than it might seem, and leveraging generic alerts is a huge
benefit for simplicity.

We use a combination of the [SRE
workbook](https://landing.google.com/sre/workbook/chapters/alerting-on-slos/)
and [SoundCloud: Alerting on SLOs like
Pros](https://developers.soundcloud.com/blog/alerting-on-slos) to form
multi-window error budget burn alerts. The term 'multi-window' indicates that
alerts are only triggered when error budget is being burned in both short and
long-term intervals: this reduces alert false positives and improves alert reset
time, causing alerts to resolve as soon as the problem has been corrected
instead of hours after.

Depending on the urgency of the detected error, we'll either page an on-call
engineer or open a ticket to handle the error budget burn in business hours. The
detection sensitivity windows are listed here:

| Alert | Long Window | Short Window | `for` Duration | Burn Rate Factor | Error Budget Consumed |
| --- | --- | --- | --- | --- | --- |
| Page | 1h | 5m | 2m | 14.4 | 2% |
| Page | 6h | 30m | 15m | 6 | 5% |
| Ticket | 1d | 2h | 1h | 3 | 10% |
| Ticket | 3d | 6h | 1h | 1 | 10% |

Every SLO created with this framework is automatically subscribed to these
alerts. Where they get routed- both who is paged, and where a ticket gets
created- depends on the team assigned to the SLO.
