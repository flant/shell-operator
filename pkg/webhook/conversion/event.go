package conversion

// type Event struct {
//	CrdName string
//	Review  *v1.ConversionReview
//	Objects []unstructured.Unstructured
//}

// Mimic a v1.ConversionReview structure but with the array of unstructured Objects
// instead of the array of runtime.RawExtension
// func (e Event) GetReview() map[string]interface{} {
//	return map[string]interface{}{
//		"kind":       e.Review.Kind,
//		"apiVersion": e.Review.APIVersion,
//		"request": map[string]interface{}{
//			"uid":               e.Review.Request.UID,
//			"desiredAPIVersion": e.Review.Request.DesiredAPIVersion,
//			"objects":           e.Objects,
//		},
//	}
//}
