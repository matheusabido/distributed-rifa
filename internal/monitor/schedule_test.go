package monitor

import (
	"reflect"
	"testing"
	"time"
)

func TestSegundosCombinados(t *testing.T) {
	want := map[int][]int{
		1: {1, 7, 13, 19, 25, 31, 37, 43, 49, 55},
		2: {3, 9, 15, 21, 27, 33, 39, 45, 51, 57},
		3: {5, 11, 17, 23, 29, 35, 41, 47, 53, 59},
	}
	for id, w := range want {
		if got := slotSeconds(id, 3); !reflect.DeepEqual(got, w) {
			t.Errorf("api%d: %v, esperava %v", id, got, w)
		}
	}
}

func TestProximoHorario(t *testing.T) {
	base := time.Date(2026, 11, 25, 20, 0, 0, 0, time.UTC)
	cases := []struct {
		now     int // segundo atual
		id      int
		proximo int // segundo esperado (pode passar de 59 = próximo minuto)
	}{
		{0, 1, 1}, {1, 1, 7}, {2, 1, 7}, {6, 1, 7}, {58, 1, 61},
		{0, 2, 3}, {3, 2, 9}, {4, 2, 9},
		{0, 3, 5}, {5, 3, 11}, {59, 3, 65},
	}
	for _, c := range cases {
		now := base.Add(time.Duration(c.now) * time.Second)
		got := nextSlot(now, c.id, 3)
		want := base.Add(time.Duration(c.proximo) * time.Second)
		if !got.Equal(want) {
			t.Errorf("api%d no segundo %d: próximo %v, esperava %v", c.id, c.now, got.Sub(base), want.Sub(base))
		}
	}
}
