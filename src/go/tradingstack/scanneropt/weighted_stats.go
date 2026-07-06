package scanneropt

import (
	"math"
	"sort"
)

// weightedMean returns the weighted mean sum(w*x)/sum(w). It returns 0 when
// the total weight is not strictly positive.
func weightedMean(values, weights []float64) float64 {
	var sum, totalWeight float64
	for i, v := range values {
		sum += weights[i] * v
		totalWeight += weights[i]
	}
	if totalWeight <= 0 {
		return 0
	}
	return sum / totalWeight
}

// weightedVariance returns the weighted population variance
// sum(w*(x-mean)^2)/sum(w) about the supplied mean. It returns 0 when the
// total weight is not strictly positive.
func weightedVariance(values, weights []float64, mean float64) float64 {
	var sumSq, totalWeight float64
	for i, v := range values {
		d := v - mean
		sumSq += weights[i] * d * d
		totalWeight += weights[i]
	}
	if totalWeight <= 0 {
		return 0
	}
	return sumSq / totalWeight
}

// pooledWeightedStdDev returns the weight-pooled standard deviation of two
// groups: sqrt((W1*var1 + W2*var2) / (W1 + W2)) where each var is the
// weighted population variance of its group and each W is the group's total
// weight. It returns 0 when the combined weight is not strictly positive.
func pooledWeightedStdDev(values1, weights1, values2, weights2 []float64) float64 {
	total1 := sumFloats(weights1)
	total2 := sumFloats(weights2)
	if total1+total2 <= 0 {
		return 0
	}

	var1 := weightedVariance(values1, weights1, weightedMean(values1, weights1))
	var2 := weightedVariance(values2, weights2, weightedMean(values2, weights2))

	return math.Sqrt((total1*var1 + total2*var2) / (total1 + total2))
}

// weightedPercentile returns the weighted p-th percentile (p in [0, 1]) of
// values: sort the (value, weight) pairs by ascending value and return the
// first value whose cumulative weight reaches at least p times the total
// weight. Deterministic for identical inputs. It returns 0 when values is
// empty or the total weight is not strictly positive.
func weightedPercentile(values, weights []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	type pair struct{ value, weight float64 }
	pairs := make([]pair, len(values))
	var totalWeight float64
	for i, v := range values {
		pairs[i] = pair{value: v, weight: weights[i]}
		totalWeight += weights[i]
	}
	if totalWeight <= 0 {
		return 0
	}

	sort.Slice(pairs, func(i, j int) bool { return pairs[i].value < pairs[j].value })

	target := p * totalWeight
	var cumulative float64
	for _, pr := range pairs {
		cumulative += pr.weight
		if cumulative >= target {
			return pr.value
		}
	}
	return pairs[len(pairs)-1].value
}

// meanStdDev returns the mean and population standard deviation of values.
// An empty slice returns (0, 0).
func meanStdDev(values []float64) (mean, stdDev float64) {
	if len(values) == 0 {
		return 0, 0
	}
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	var sumSq float64
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return mean, math.Sqrt(sumSq / float64(len(values)))
}

func sumFloats(values []float64) float64 {
	var total float64
	for _, v := range values {
		total += v
	}
	return total
}
