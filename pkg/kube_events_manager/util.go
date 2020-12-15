package kube_events_manager

import (
	"fmt"
	"math/rand"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"

	. "github.com/flant/shell-operator/pkg/kube_events_manager/types"
)

// ResourceId describes object with namespace, kind and name
//
// Change with caution, as this string is used for sorting objects and snapshots.
func ResourceId(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s", obj.GetNamespace(), obj.GetKind(), obj.GetName())
}

func FormatLabelSelector(selector *metav1.LabelSelector) (string, error) {
	res, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return "", err
	}

	return res.String(), nil
}

func FormatFieldSelector(selector *FieldSelector) (string, error) {
	if selector == nil || selector.MatchExpressions == nil {
		return "", nil
	}

	requirements := make([]fields.Selector, 0)

	for _, req := range selector.MatchExpressions {
		switch req.Operator {
		case "=", "==", "Equals":
			requirements = append(requirements, fields.OneTermEqualSelector(req.Field, req.Value))
		case "!=", "NotEquals":
			requirements = append(requirements, fields.OneTermNotEqualSelector(req.Field, req.Value))
		default:
			return "", fmt.Errorf("%s%s%s: operator '%s' is not recognized", req.Field, req.Operator, req.Value, req.Operator)
		}
	}

	return fields.AndSelectors(requirements...).String(), nil
}

const ResyncPeriodMedian = time.Duration(3) * time.Hour
const ResyncPeriodSpread = time.Duration(2) * time.Hour
const ResyncPeriodGranularity = time.Duration(5) * time.Minute
const ResyncPeriodJitterGranularity = time.Duration(15) * time.Second

// RandomizedResyncPeriod returns a time.Duration between 2 hours and 4 hours with jitter and granularity
func RandomizedResyncPeriod() time.Duration {
	spreadCount := ResyncPeriodSpread.Nanoseconds() / ResyncPeriodGranularity.Nanoseconds()
	rndSpreadDelta := time.Duration(rand.Int63n(spreadCount)) * ResyncPeriodGranularity
	jitterCount := ResyncPeriodGranularity.Nanoseconds() / ResyncPeriodJitterGranularity.Nanoseconds()
	rndJitterDelta := time.Duration(rand.Int63n(jitterCount)) * ResyncPeriodJitterGranularity

	return ResyncPeriodMedian - (ResyncPeriodSpread / 2) + rndSpreadDelta + rndJitterDelta
}
