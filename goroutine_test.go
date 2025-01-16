package redisson

import "testing"

func TestGetId(t *testing.T) {
	id, err := getId()
	if err != nil {
		t.Error(err)
	}
	t.Log(id)
}
