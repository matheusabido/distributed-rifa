package monitor

import "time"

// Horários de checagem das SEGUIDORAS (a mestre checa a cada 2s, sem
// horário fixo; ver masterInterval em monitor.go).
//
// Cada API tem a sua vez, separadas por 2 segundos:
//
//	período = 2 × (número de APIs)       -> 3 APIs: a cada 6s
//	segundo = 2 × ID - 1                  -> API 1: 1, API 2: 3, API 3: 5
//
// Com 3 APIs:
//
//	API 1: 1, 7, 13, 19, 25...
//	API 2: 3, 9, 15, 21, 27...
//	API 3: 5, 11, 17, 23, 29...
//
// Ou seja: as seguidoras se revezam conferindo se a mestre responde, e se
// uma delas cair as outras continuam conferindo. Os horários usam o relógio do computador, que é
// o mesmo para todos os containers.

func period(totalAPIs int) time.Duration {
	return time.Duration(2*totalAPIs) * time.Second
}

func offset(id, totalAPIs int) time.Duration {
	return time.Duration(2*id-1) * time.Second % period(totalAPIs)
}

// NextSlot devolve o próximo horário (depois de "now") em que esta API checa.
func NextSlot(now time.Time, id, totalAPIs int) time.Time {
	p := period(totalAPIs)
	// Truncate alinha no múltiplo de p (ex: :00, :06, :12...), porque o
	// período divide 60 quando há 1, 2, 3, 5, 6, 10, 15 ou 30 APIs.
	t := now.Truncate(p).Add(offset(id, totalAPIs))
	if !t.After(now) {
		t = t.Add(p)
	}
	return t
}

// SlotSeconds lista os segundos de um minuto em que a API checa (para log).
func SlotSeconds(id, totalAPIs int) []int {
	var out []int
	p := int(period(totalAPIs) / time.Second)
	for s := int(offset(id, totalAPIs) / time.Second); s < 60; s += p {
		out = append(out, s)
	}
	return out
}
