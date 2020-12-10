package validating_webhook

type ReviewResponse struct {
	Allow  bool
	Result *ReviewResponseResult
}

type ReviewResponseResult struct {
	Code    int
	Message string
}
