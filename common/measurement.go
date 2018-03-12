package common

func (dp DataPoint) VisibleImpairment() float64 {
	return 1 - dp.WorkRatio - dp.SleepRatio
}
