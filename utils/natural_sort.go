package utils

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// NaturalSort implements natural sorting for strings with numbers
// Example: "Item 1", "Item 2", "Item 10" instead of "Item 1", "Item 10", "Item 2"

// splitIntoTokens splits a string into text and number tokens
func splitIntoTokens(s string) []token {
	var tokens []token
	var currentText strings.Builder
	var currentNumber strings.Builder
	inNumber := false

	for _, r := range s {
		if unicode.IsDigit(r) {
			if !inNumber && currentText.Len() > 0 {
				// Save accumulated text
				tokens = append(tokens, token{isNumber: false, text: currentText.String()})
				currentText.Reset()
			}
			inNumber = true
			currentNumber.WriteRune(r)
		} else {
			if inNumber && currentNumber.Len() > 0 {
				// Save accumulated number
				num, _ := strconv.ParseInt(currentNumber.String(), 10, 64)
				tokens = append(tokens, token{isNumber: true, number: num, text: currentNumber.String()})
				currentNumber.Reset()
			}
			inNumber = false
			currentText.WriteRune(r)
		}
	}

	// Save remaining
	if currentNumber.Len() > 0 {
		num, _ := strconv.ParseInt(currentNumber.String(), 10, 64)
		tokens = append(tokens, token{isNumber: true, number: num, text: currentNumber.String()})
	}
	if currentText.Len() > 0 {
		tokens = append(tokens, token{isNumber: false, text: currentText.String()})
	}

	return tokens
}

type token struct {
	isNumber bool
	number   int64
	text     string
}

// NaturalLess compares two strings using natural sorting
func NaturalLess(s1, s2 string) bool {
	// Convert to lowercase for case-insensitive comparison
	tokens1 := splitIntoTokens(strings.ToLower(s1))
	tokens2 := splitIntoTokens(strings.ToLower(s2))

	// Compare token by token
	for i := 0; i < len(tokens1) && i < len(tokens2); i++ {
		t1 := tokens1[i]
		t2 := tokens2[i]

		// Both are numbers - compare numerically
		if t1.isNumber && t2.isNumber {
			if t1.number != t2.number {
				return t1.number < t2.number
			}
			continue
		}

		// Both are text - compare alphabetically
		if !t1.isNumber && !t2.isNumber {
			if t1.text != t2.text {
				return t1.text < t2.text
			}
			continue
		}

		// One is number, one is text - numbers come first
		if t1.isNumber && !t2.isNumber {
			return true
		}
		if !t1.isNumber && t2.isNumber {
			return false
		}
	}

	// If all tokens are equal, shorter string comes first
	return len(tokens1) < len(tokens2)
}

// SortGroupsNaturally sorts a map of groups by name using natural sorting
func SortGroupsNaturally(groups map[string]string) []struct {
	JID  string
	Name string
} {
	// Convert map to slice
	type groupPair struct {
		JID  string
		Name string
	}

	var pairs []groupPair
	for jid, name := range groups {
		pairs = append(pairs, groupPair{JID: jid, Name: name})
	}

	// Sort using natural comparison
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if !NaturalLess(pairs[i].Name, pairs[j].Name) {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// Convert back to the expected format
	var result []struct {
		JID  string
		Name string
	}

	for _, pair := range pairs {
		result = append(result, struct {
			JID  string
			Name string
		}{JID: pair.JID, Name: pair.Name})
	}

	return result
}

// ExtractNumbersFromString extracts all numbers from a string for sorting purposes
func ExtractNumbersFromString(s string) []int {
	re := regexp.MustCompile(`\d+`)
	matches := re.FindAllString(s, -1)

	var numbers []int
	for _, match := range matches {
		if num, err := strconv.Atoi(match); err == nil {
			numbers = append(numbers, num)
		}
	}

	return numbers
}

// CompareGroupNames compares two group names naturally
// Returns: -1 if name1 < name2, 0 if equal, 1 if name1 > name2
func CompareGroupNames(name1, name2 string) int {
	if NaturalLess(name1, name2) {
		return -1
	}
	if NaturalLess(name2, name1) {
		return 1
	}
	return 0
}
