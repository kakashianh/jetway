package inventory

import (
	"encoding/binary"
	"errors"
	"math"
)

// Overbooking: how many bookings a cabin may hold when some of the
// passengers who hold them will not come.
//
// The terms and the models are those of Peter Belobaba's lecture "Flight
// Overbooking: Models and Practice" (MIT OpenCourseWare 16.75J Airline
// Management, lecture 19, 2006). A cabin has a physical capacity CAP and
// an authorised capacity AU, and the confirmed bookings BKD never pass AU.
// Of the bookings at departure, NS do not come and SU do, so the no-show
// rate is NSR = NS/BKD and the show-up rate is SUR = 1 - NSR. A passenger
// with a confirmed booking who comes and finds no seat is a denied
// boarding, DB. A seat that leaves empty from a cabin that had turned
// bookings away is spoilage, SP.
//
// An empty seat is therefore one of three things, and they are kept
// apart. A seat nobody booked is unsold: the forecast's matter. A seat
// booked by a passenger who did not come is a no-show: this record's
// matter. A no-show seat on a cabin that was closed at its limit is
// spoiled: the cost the limit is set against.

// Shows is a cabin's record of who came, over its past departures: of the
// bookings held at each departure, how many passengers showed up. The
// record is weighed down at each departure, as a History is, so the rate
// follows a flight whose passengers change. The zero value is an empty
// record.
type Shows struct {
	// Memory is how many departures the record remembers, as the span of
	// an exponential average. Zero is 26.
	Memory float64

	n      int
	weight float64 // bookings, weighed down
	shown  float64 // show-ups, weighed down
	square float64 // each departure's show-ups squared over its bookings
	spread float64 // the squares of the weights
}

// Departures is how many departures the record holds.
func (s *Shows) Departures() int { return s.n }

// Record adds one departure: the confirmed bookings the cabin held when
// the flight closed and how many of them did not come. Go-shows and
// standbys are not bookings and are not counted. A departure with no
// bookings says nothing and is not recorded.
func (s *Shows) Record(booked, noShows int) {
	if booked <= 0 {
		return
	}
	noShows = min(max(noShows, 0), booked)
	memory := s.Memory
	if memory <= 0 {
		memory = 26
	}
	keep := 1 - 2/(memory+1)
	s.weight *= keep
	s.shown *= keep
	s.square *= keep
	s.spread *= keep * keep
	b, su := float64(booked), float64(booked-noShows)
	s.weight += b
	s.shown += su
	s.square += su * su / b
	s.spread += b * b
	s.n++
}

// Rate is the show-up rate the record gives: its mean, weighted by each
// departure's bookings, and the standard deviation of the rate from one
// departure to the next. ok is false for an empty record. The deviation
// is zero until the record holds 2 departures.
//
// The deviation is what the record saw. A caller with a short record
// does well not to trust a deviation under the binomial's,
// sqrt(SUR*(1-SUR)/BKD), which is the least a cabin of independent
// passengers can have.
func (s *Shows) Rate() (mean, deviation float64, ok bool) {
	if s.weight <= 0 {
		return 0, 0, false
	}
	mean = s.shown / s.weight
	// The weighted variance of the rates, corrected for the weights: with
	// equal weights the divisor is the n-1 of a sample variance.
	free := 1 - s.spread/(s.weight*s.weight)
	if free <= 1e-9 {
		return mean, 0, true
	}
	v := (s.square/s.weight - mean*mean) / free
	if v < 0 {
		v = 0
	}
	return mean, math.Sqrt(v), true
}

// showsMagic leads a Shows record's binary form.
const showsMagic = "JWS1"

// MarshalBinary is the record in a compact form. Memory is the caller's
// and is not in it.
func (s *Shows) MarshalBinary() ([]byte, error) {
	b := []byte(showsMagic)
	b = binary.AppendUvarint(b, uint64(s.n))
	for _, v := range []float64{s.weight, s.shown, s.square, s.spread} {
		b = binary.LittleEndian.AppendUint64(b, math.Float64bits(v))
	}
	return b, nil
}

// UnmarshalBinary reads the form MarshalBinary writes.
func (s *Shows) UnmarshalBinary(b []byte) error {
	bad := errors.New("inventory: not a show-up record")
	if len(b) < len(showsMagic) || string(b[:len(showsMagic)]) != showsMagic {
		return bad
	}
	b = b[len(showsMagic):]
	n, k := binary.Uvarint(b)
	if k <= 0 || len(b[k:]) != 32 {
		return bad
	}
	b = b[k:]
	var v [4]float64
	for i := range v {
		v[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[8*i:]))
		if math.IsNaN(v[i]) || math.IsInf(v[i], 0) || v[i] < 0 {
			return bad
		}
	}
	s.n, s.weight, s.shown, s.square, s.spread = int(n), v[0], v[1], v[2], v[3]
	return nil
}

// whole is the largest whole number of bookings not over x. A limit is
// rounded down, because the model's promise holds at the limit and not
// above it.
func whole(x float64, capacity int) int {
	if math.IsNaN(x) || x < float64(capacity) {
		return capacity
	}
	if x > float64(3*capacity) {
		return 3 * capacity
	}
	return int(math.Floor(x + 1e-9))
}

// Deterministic is the authorised capacity that fills the cabin exactly
// when the bookings reach it and the mean show-up rate holds:
// AU = CAP / (1 - NSR). The lecture's second model.
func Deterministic(capacity int, showRate float64) int {
	if capacity <= 0 || showRate <= 0 || showRate >= 1 {
		return max(capacity, 0)
	}
	return whole(float64(capacity)/showRate, capacity)
}

// Probabilistic is the authorised capacity that keeps the show-ups within
// the cabin with a given confidence when the bookings reach it, the
// show-up rate being normal with a mean and a standard deviation:
// AU = CAP / (SUR + z*STD), z the normal quantile of the confidence. The
// lecture's third model, where the confidence is 0.95 and z is 1.645.
//
// The lecture's extensions go in the capacity. A carrier that expects
// volunteers adds the denied boardings it will accept. A carrier that
// seats an oversold main cabin in the empty seats of the cabin in front
// adds the seats it expects empty there.
func Probabilistic(capacity int, showRate, deviation, confidence float64) int {
	if capacity <= 0 || showRate <= 0 {
		return max(capacity, 0)
	}
	z := normalQuantile(confidence)
	if math.IsInf(z, 0) || z < 0 {
		z = 0
	}
	rate := showRate + z*math.Max(deviation, 0)
	if rate >= 1 {
		return capacity
	}
	return whole(float64(capacity)/rate, capacity)
}

// Expected is the denied boardings and the spoiled seats to expect of a
// cabin whose bookings reach an authorised capacity, the show-up rate
// being normal with a mean and a standard deviation. The show-ups are
// then normal about AU*SUR with deviation AU*STD.
func Expected(capacity, authorized int, showRate, deviation float64) (denied, spoiled float64) {
	c, mu := float64(capacity), float64(authorized)*showRate
	sigma := float64(authorized) * math.Max(deviation, 0)
	if sigma <= 0 {
		return math.Max(mu-c, 0), math.Max(c-mu, 0)
	}
	d := (mu - c) / sigma
	denied = (mu-c)*0.5*math.Erfc(-d/math.Sqrt2) + sigma*math.Exp(-d*d/2)/math.Sqrt(2*math.Pi)
	if denied < 0 {
		denied = 0
	}
	return denied, math.Max(denied-(mu-c), 0)
}

// CostBased is the authorised capacity with the least expected cost of
// denied boardings and spoilage together, each at its own cost for one
// passenger or seat: the lecture's fourth model, a search over AU from
// the capacity up. The cost of a denied boarding is the compensation,
// the care and the seat on a later flight. The cost of a spoiled seat is
// the revenue a booking turned away would have brought.
//
// The expected cost is convex in AU under Expected's model, so the search
// stops at the first limit that costs more than the one before it.
func CostBased(capacity int, showRate, deviation, deniedCost, spoiledCost float64) int {
	if capacity <= 0 || showRate <= 0 || showRate >= 1 || spoiledCost <= 0 {
		return max(capacity, 0)
	}
	if deniedCost <= 0 {
		return 3 * capacity
	}
	cost := func(au int) float64 {
		db, sp := Expected(capacity, au, showRate, deviation)
		return deniedCost*db + spoiledCost*sp
	}
	best, least := capacity, cost(capacity)
	for au := capacity + 1; au <= 3*capacity; au++ {
		c := cost(au)
		if c >= least {
			break
		}
		best, least = au, c
	}
	return best
}

// Departure is one departure of a cabin, counted as an overbooking model
// counts it.
type Departure struct {
	// Capacity is the seats, and Authorized the bookings the cabin was
	// allowed. Authorized under Capacity counts as Capacity.
	Capacity, Authorized int
	// Booked is the confirmed bookings at departure and NoShows those of
	// them who did not come.
	Booked, NoShows int
	// GoShows and Standbys came without a confirmed booking on the
	// flight. They board only when seats are left after the bookings.
	GoShows, Standbys int
}

// ShowUps is the booked passengers who came: SU = BKD - NS.
func (d Departure) ShowUps() int { return max(d.Booked-min(max(d.NoShows, 0), d.Booked), 0) }

// Denied is the booked passengers who came and had no seat.
func (d Departure) Denied() int { return max(d.ShowUps()-d.Capacity, 0) }

// Boarded is the passengers the cabin left with:
// PAX = min(BKD - NS + GS + SB, CAP).
func (d Departure) Boarded() int {
	return min(d.ShowUps()+max(d.GoShows, 0)+max(d.Standbys, 0), max(d.Capacity, 0))
}

// Spoiled is the seats that left empty from a cabin whose bookings had
// reached its limit: SP = CAP - PAX, only when BKD = AU. An empty seat on
// a cabin that never reached its limit is unsold and is not spoilage,
// because no limit turned a booking away.
func (d Departure) Spoiled() int {
	if d.Booked < max(d.Authorized, d.Capacity) {
		return 0
	}
	return max(d.Capacity-d.Boarded(), 0)
}
