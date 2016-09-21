package benchlib

import "math"

// Mean returns the mean of an integer array as a float
func Mean(nums []int64) (mean float64) {
	if len(nums) == 0 {
		return 0.0
	}
	for _, n := range nums {
		mean += float64(n)
	}
	return mean / float64(len(nums))
}

// StandardDeviation returns the standard deviation of the slice
// as a float
func StandardDeviation(nums []int64) (dev float64) {
	if len(nums) == 0 {
		return 0.0
	}

	m := Mean(nums)
	for _, n := range nums {
		dev += (float64(n) - m) * (float64(n) - m)
	}
	dev = math.Pow(dev/float64(len(nums)), 0.5)
	return dev
}

// NormalConfidenceInterval returns the 95% confidence interval for the mean
// as two float values, the lower and the upper bounds and assuming a normal
// distribution
func NormalConfidenceInterval(nums []int64) (lower float64, upper float64) {
	conf := 1.95996 // 95% confidence for the mean, http://bit.ly/Mm05eZ
	mean := Mean(nums)
	dev := StandardDeviation(nums) / math.Sqrt(float64(len(nums)))
	return mean - dev*conf, mean + dev*conf
}
