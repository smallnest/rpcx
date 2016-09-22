package clientselector

// Weighted is a wrapped server with  weight
type Weighted struct {
	Server          interface{}
	Weight          int
	CurrentWeight   int
	EffectiveWeight int
}

func (w *Weighted) fail() {
	w.EffectiveWeight -= w.Weight
	if w.EffectiveWeight < 0 {
		w.EffectiveWeight = 0
	}
}

//https://github.com/phusion/nginx/commit/27e94984486058d73157038f7950a0a36ecc6e35
func nextWeighted(servers []*Weighted) (best *Weighted) {
	total := 0

	for i := 0; i < len(servers); i++ {
		w := servers[i]

		if w == nil {
			continue
		}
		//if w is down, continue

		w.CurrentWeight += w.EffectiveWeight
		total += w.EffectiveWeight
		if w.EffectiveWeight < w.Weight {
			w.EffectiveWeight++
		}

		if best == nil || w.CurrentWeight > best.CurrentWeight {
			best = w
		}

	}

	if best == nil {
		return nil
	}

	best.CurrentWeight -= total
	return best
}
