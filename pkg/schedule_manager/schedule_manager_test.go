package schedulemanager

import (
	"context"
	"testing"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/shell-operator/pkg/schedule_manager/types"
)

func Test_ScheduleManager_Add(t *testing.T) {
	sm := NewScheduleManager(context.Background(), log.NewNop())

	expectations := []struct {
		testName string
		crontab  string
		id       string
		err      string
	}{
		{
			"crontab",
			"* * * * *",
			"* * * * *",
			"",
		},
		{
			"incorrect crontab format (value)",
			"* * * 22 *",
			"",
			"End of range (22) above maximum (12): 22",
		},
		{
			"incorrect crontab format (fields)",
			"incorrect",
			"",
			"Expected 5 or 6 fields, found 1: incorrect",
		},
	}

	for _, expectation := range expectations {
		t.Run(expectation.testName, func(_ *testing.T) {
			sm.Add(types.ScheduleEntry{Crontab: expectation.crontab, Id: expectation.id})

			// if expectation.err != "" {
			//	if err == nil {
			//		t.Errorf("Expected specific error: %s", expectation.err)
			//	} else {
			//		assert.Equal(t, expectation.err, err.Error())
			//	}
			// } else if err != nil {
			//	t.Error(err)
			//}
		})
	}

	// t.Run("1 crontab == 1 job", func(t *testing.T) {
	//	id1, err := sm.Add("* */2 * * *")
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	id2, err := sm.Add("* */2 * * *")
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	assert.Equal(t, id1, id2)
	// })
}

// TODO rewrite with faked time
// func Test_ScheduleManager_Run(t *testing.T) {
//	sm := NewScheduleManager()
//	sm.ScheduleCh = make(chan string)
//
//	expectations := []struct {
//		crontab string
//		counter int
//	}{
//		{
//			"*/2 * * * * *",
//			3,
//		},
//		{
//			"* * * * * *",
//			6,
//		},
//	}
//
//	for _, expectation := range expectations {
//		_, err := sm.Add(expectation.crontab)
//		if err != nil {
//			t.Fatal(err)
//		}
//	}
//
//	timer := time.NewTimer(time.Second * 6)
//	entryCounters := make(map[string]int)
//	sm.Run()
//
//infinity:
//	for {
//		select {
//		case crontab := <-ScheduleCh:
//			entryCounters[crontab] += 1
//		case <-timer.C:
//			sm.stop()
//			break infinity
//		}
//	}
//
//	for _, expectation := range expectations {
//		assert.Equal(t, entryCounters[expectation.crontab], expectation.counter)
//	}
//}

// TODO rewrite with faked time
// func Test_ScheduleManager_Remove(t *testing.T) {
//	t.Run("base", func(t *testing.T) {
//		sm := NewScheduleManager()
//
//		ScheduleCh = make(chan string)
//		expectation := struct {
//			crontab string
//			counter int
//		}{
//			"* * * * * *",
//			1,
//		}
//
//		_, err := sm.Add(expectation.crontab)
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		timer := time.NewTimer(time.Second * 5)
//		entryCounters := make(map[string]int)
//		sm.Run()
//
//	infinity:
//		for {
//			select {
//			case crontab := <-ScheduleCh:
//				entryCounters[crontab] += 1
//				if err := sm.Remove(crontab); err != nil {
//					t.Fatal(err)
//				}
//			case <-timer.C:
//				sm.stop()
//				break infinity
//			}
//		}
//
//		assert.Equal(t, entryCounters[expectation.crontab], expectation.counter)
//	})
//
//	t.Run("not found", func(t *testing.T) {
//		sm := NewScheduleManager()
//		expectedError := "schedule manager entry '* * * * *' not found"
//		err := sm.Remove("* * * * *")
//		if err == nil {
//			t.Errorf("Expected specific error: %s", expectedError)
//		} else {
//			assert.Equal(t, expectedError, err.Error())
//		}
//	})
//}
