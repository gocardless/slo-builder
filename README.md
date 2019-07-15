# slo-alerts

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

## `BaseSLO`

Every SLO has some common behaviour, as represented by the `BaseSLO` type. An
example core SLO definition would be:

```go
BaseSLO{
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
job:slo_definition:none{name="MarkPaymentsAsPaidMeetsDeadline",error_budget="0.1"} 1.0
job:slo_error_budget:ratio{name="MarkPaymentsAsPaidMeetsDeadline"} 0.1
```

## `BatchProcessingSLO`

We'll use an example of a process that transitions many payments into a paid
state as a batch process for which we want to apply an SLO.

```go
MarkPaymentsAsPaidMeetsDeadline = BatchProcessingSLO{
  BaseSLO:  BaseSLO{"MarkPaymentsAsPaidMeetsDeadline", 0.1},
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
