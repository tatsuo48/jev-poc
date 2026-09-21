package jev

import (
	"context"
	"math"
	"os"
	"testing"
	"time"
)

// Runs only when TYPESAFE_API_KEY is set; it spends exactly one API call.
func TestIntegrationRealAPI(t *testing.T) {
	if os.Getenv("TYPESAFE_API_KEY") == "" {
		t.Skip("TYPESAFE_API_KEY is not set")
	}
	c, err := NewFromEnv(1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	criteria := map[string]string{
		"billing":   "Payments, invoices and refunds.",
		"technical": "Bugs, errors and outages.",
	}
	resp, err := c.Ask(ctx, map[string]any{"message": "I was charged twice this month."}, map[string]Question{
		"department": {Type: "choice", Instructions: "Which department should handle this message?", Criteria: criteria},
	})
	if err != nil {
		t.Fatal(err)
	}
	ans, ok := resp.Answers["department"]
	if !ok {
		t.Fatalf("no answer for the question: %+v", resp)
	}
	if _, ok := criteria[ans.Choice]; !ok {
		t.Errorf("choice %q is not one of the options", ans.Choice)
	}
	sum := 0.0
	for _, p := range ans.Probabilities {
		sum += p
	}
	if len(ans.Probabilities) != 2 || math.Abs(sum-1) > 0.01 {
		t.Errorf("probabilities = %v", ans.Probabilities)
	}
	if resp.Usage.InputTokens == 0 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}
