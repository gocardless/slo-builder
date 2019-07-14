package main

import "time"

var (
	MarkPaymentsAsPaidMeetsDeadline = BatchProcessingSLO{
		BaseSLO:  BaseSLO{"MarkPaymentsAsPaid meets deadline", 0.1},
		Deadline: time.Duration(2) * time.Hour,
		Volume: `
1.5 * max_over_time(
  (
    sum by (namespace, release) (
      increase(paysvc_mark_payments_as_paid_marked_as_paid_total[8h])
    )
  )[60d:1h]
)
		`,
		Throughput: `
sum by (namespace, release) (
  rate(paysvc_mark_payments_as_paid_marked_as_paid_total[1m])
) > 0
		`,
	}
)

func init() {
	MustRegister(MarkPaymentsAsPaidMeetsDeadline.Rules()...)
}
