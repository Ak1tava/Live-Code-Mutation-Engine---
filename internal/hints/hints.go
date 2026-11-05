// Package hints holds the hint type attached to an audit session.
package hints

// Hint is an explanation and a guiding question for one mutation.
type Hint struct {
	MutationID  string `json:"mutationId"`
	Explanation string `json:"explanation"`
	Question    string `json:"question"`
	Source      string `json:"source"`
}
