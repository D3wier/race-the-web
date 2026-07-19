package analyzer

import (
	"github.com/D3wier/race-the-web/pkg/racer"
)

type Analysis struct {
	RaceDetected    bool     `json:"race_detected"`
	Confidence      string   `json:"confidence"`
	SuccessCount    int      `json:"success_count"`
	ExpectedSuccess int      `json:"expected_success"`
	Indicators      []string `json:"indicators"`
}

func Analyze(results []racer.RoundResult) Analysis {
	a := Analysis{
		ExpectedSuccess: len(results),
	}

	statusMap := make(map[int]int)
	bodyMap := make(map[string]int)
	var successCounts []int

	for _, round := range results {
		roundSuccess := 0
		for _, resp := range round.Responses {
			if resp.Error != "" {
				continue
			}
			statusMap[resp.StatusCode]++
			bodyMap[resp.Body]++
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				roundSuccess++
			}
		}
		successCounts = append(successCounts, roundSuccess)
		a.SuccessCount += roundSuccess
	}

	if a.SuccessCount > a.ExpectedSuccess {
		a.RaceDetected = true

		ratio := float64(a.SuccessCount) / float64(a.ExpectedSuccess)
		if ratio >= 3.0 {
			a.Confidence = "high"
			a.Indicators = append(a.Indicators, "Multiple requests consistently succeed")
		} else if ratio >= 1.5 {
			a.Confidence = "medium"
			a.Indicators = append(a.Indicators, "Some requests succeed beyond expected count")
		} else {
			a.Confidence = "low"
			a.Indicators = append(a.Indicators, "Marginal extra successes detected")
		}
	}

	if len(bodyMap) > 2 {
		a.Indicators = append(a.Indicators, "Multiple distinct response bodies observed")
	}

	return a
}
