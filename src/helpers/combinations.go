package helpers

import "gonum.org/v1/gonum/stat/combin"

func GetCombinations(set []string, combinationLength int) [][]string {
	combinations := make([][]string, 0)

	gen := combin.NewCombinationGenerator(len(set), combinationLength)

	for gen.Next() {
		numsCombination := gen.Combination(nil)

		combination := make([]string, 0)

		for _, num := range numsCombination {
			combination = append(combination, set[num])
		}

		combinations = append(combinations, combination)
	}

	return combinations
}
