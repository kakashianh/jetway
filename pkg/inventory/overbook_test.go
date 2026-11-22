package inventory

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/adamf/jetway/pkg/avail"
)

// The figures are those of Peter Belobaba's lecture "Flight Overbooking:
// Models and Practice" (MIT OpenCourseWare 16.75J, lecture 19, 2006).
//
// The deterministic model: "For CAP=100 and NSR=0.20, then
// AU = 100/(1-.20) = 125". The probabilistic model: "with STD= 0.05:
// AU = 100 / (1-0.20 + 1.645*0.05) = 113".
func TestOverbookingAgainstBelobabaLecture(t *testing.T) {
	if got := Deterministic(100, 0.80); got != 125 {
		t.Errorf("deterministic: AU = %d, want 125", got)
	}
	if got := Probabilistic(100, 0.80, 0.05, 0.95); got != 113 {
		t.Errorf("probabilistic: AU = %d, want 113", got)
	}
	// "The larger STD, the larger the denominator and the lower the
	// optimal AU": 100/(0.80+1.645*0.08) = 107.3.
	if got := Probabilistic(100, 0.80, 0.08, 0.95); got != 107 {
		t.Errorf("probabilistic with STD 0.08: AU = %d, want 107", got)
	}
	// Extension 1: a lower confidence gives a higher limit. At 0.80 the
	// quantile is 0.8416 and AU = 100/(0.80+0.0421) = 118.8.
	if got := Probabilistic(100, 0.80, 0.05, 0.80); got != 118 {
		t.Errorf("probabilistic at 0.80: AU = %d, want 118", got)
	}
	// Extension 2: 3 volunteers accepted put 103 in the numerator, and
	// AU = 103/0.88225 = 116.7.
	if got := Probabilistic(103, 0.80, 0.05, 0.95); got != 116 {
		t.Errorf("probabilistic with 3 volunteers: AU = %d, want 116", got)
	}
	// With no deviation the probabilistic model is the deterministic one.
	if got := Probabilistic(100, 0.80, 0, 0.95); got != 125 {
		t.Errorf("probabilistic with no deviation: AU = %d, want 125", got)
	}
	// A cabin where everybody comes is not overbooked.
	if got := Deterministic(100, 1); got != 100 {
		t.Errorf("everybody comes: AU = %d, want 100", got)
	}
}

// The lecture's cost-based example is 2 charts: "CAP=120 NSR = 0.15,
// Sigma =0.08" and "DB Cost = $50, SP Cost = $100". The figures here were
// read from the charts by eye, so each has the width of a reading.
//
//   - At AU 120 the spoilage is just under 18 seats and no boarding is
//     denied.
//   - The 2 curves cross between AU 141 and 142, a little over 4.
//   - At AU 170 the denied boardings are just under 25.
//   - The total cost is least, at about $600, between AU 143 and 148.
//
// By hand, under the normal model: at AU 120 the spoilage is
// 120 * (0.15*Phi(1.875) + 0.08*phi(1.875)) = 120 * 0.15094 = 18.1. The
// curves cross where AU*0.85 = 120, at AU 141.2, at
// 141.2 * 0.08 * phi(0) = 4.5. At AU 170 the mean is 144.5 and the
// deviation 13.6, so d = 1.8015 and the denied boardings are
// 24.5*0.9642 + 13.6*0.0788 = 24.7.
func TestCostBasedOverbookingAgainstBelobabaCharts(t *testing.T) {
	near := func(what string, got, want, width float64) {
		t.Helper()
		if math.Abs(got-want) > width {
			t.Errorf("%s = %.2f, want %.2f within %.2f", what, got, want, width)
		}
	}
	db, sp := Expected(120, 120, 0.85, 0.08)
	near("spoilage at AU 120", sp, 18.1, 0.1)
	near("denied boardings at AU 120", db, 0.1, 0.1)
	db141, sp141 := Expected(120, 141, 0.85, 0.08)
	db142, sp142 := Expected(120, 142, 0.85, 0.08)
	if !(db141 < sp141 && db142 > sp142) {
		t.Errorf("the curves cross outside AU 141 to 142: %.2f and %.2f, then %.2f and %.2f", db141, sp141, db142, sp142)
	}
	near("denied boardings at AU 141", db141, 4.4, 0.3)
	db, _ = Expected(120, 170, 0.85, 0.08)
	near("denied boardings at AU 170", db, 24.7, 0.1)

	au := CostBased(120, 0.85, 0.08, 50, 100)
	if au < 143 || au > 148 {
		t.Errorf("cost-based AU = %d, want 143 to 148", au)
	}
	db, sp = Expected(120, au, 0.85, 0.08)
	if cost := 50*db + 100*sp; cost < 580 || cost > 660 {
		t.Errorf("least cost = $%.0f, want about $600", cost)
	}
	// The search found the least: the limits on each side cost more.
	at := func(n int) float64 { d, s := Expected(120, n, 0.85, 0.08); return 50*d + 100*s }
	if at(au-1) < at(au) || at(au+1) < at(au) {
		t.Errorf("AU %d is not the least: %.1f, %.1f, %.1f", au, at(au-1), at(au), at(au+1))
	}
	// A dearer denied boarding lowers the limit, a dearer empty seat
	// raises it, and a seat worth nothing is not overbooked.
	if hi := CostBased(120, 0.85, 0.08, 400, 100); hi >= au {
		t.Errorf("denied boarding at $400: AU %d, want under %d", hi, au)
	}
	if lo := CostBased(120, 0.85, 0.08, 50, 400); lo <= au {
		t.Errorf("empty seat at $400: AU %d, want over %d", lo, au)
	}
	if got := CostBased(120, 0.85, 0.08, 50, 0); got != 120 {
		t.Errorf("an empty seat that costs nothing: AU %d, want 120", got)
	}
}

// 3 departures of 100 bookings, with 90, 80 and 85 show-ups, remembered
// in full. The mean rate is 255/300 = 0.85. The rates 0.90, 0.80 and 0.85
// have the sample variance (0.0025+0.0025+0)/2 = 0.0025, a deviation of
// 0.05. The probabilistic limit is then 100/(0.85+1.645*0.05) = 107.3.
func TestShowsReadsItsRecord(t *testing.T) {
	s := &Shows{Memory: 1e12}
	if _, _, ok := s.Rate(); ok {
		t.Fatal("an empty record gave a rate")
	}
	s.Record(100, 10)
	if mean, dev, ok := s.Rate(); !ok || math.Abs(mean-0.90) > 1e-9 || dev != 0 {
		t.Fatalf("after 1 departure: %.4f, %.4f, %v; want 0.90, 0, true", mean, dev, ok)
	}
	s.Record(100, 20)
	s.Record(100, 15)
	s.Record(0, 0) // says nothing
	mean, dev, ok := s.Rate()
	if !ok || s.Departures() != 3 {
		t.Fatalf("departures = %d, ok %v", s.Departures(), ok)
	}
	if math.Abs(mean-0.85) > 1e-6 || math.Abs(dev-0.05) > 1e-6 {
		t.Fatalf("rate = %.5f, deviation %.5f; want 0.85 and 0.05", mean, dev)
	}
	if au := Probabilistic(100, mean, dev, 0.95); au != 107 {
		t.Errorf("AU = %d, want 107", au)
	}

	// The weights are the bookings. A departure of 300 bookings with 240
	// show-ups and one of 100 with 100: the mean is 340/400 = 0.85. The
	// weighted variance is (300*0.05^2 + 100*0.15^2)/400 = 0.0075, over
	// 1 - (300^2+100^2)/400^2 = 0.375, which is 0.02: a deviation of
	// 0.14142.
	w := &Shows{Memory: 1e12}
	w.Record(300, 60)
	w.Record(100, 0)
	mean, dev, _ = w.Rate()
	if math.Abs(mean-0.85) > 1e-6 || math.Abs(dev-0.141421) > 1e-5 {
		t.Fatalf("weighted: rate = %.5f, deviation %.5f; want 0.85 and 0.14142", mean, dev)
	}

	// The record across a restart.
	b, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	back := &Shows{Memory: 1e12}
	if err := back.UnmarshalBinary(b); err != nil {
		t.Fatal(err)
	}
	m2, d2, _ := back.Rate()
	if back.Departures() != 3 || math.Abs(m2-0.85) > 1e-6 || math.Abs(d2-0.05) > 1e-6 {
		t.Errorf("after the round trip: %d departures, %.5f, %.5f", back.Departures(), m2, d2)
	}
	if err := back.UnmarshalBinary(b[:len(b)-1]); err == nil {
		t.Error("a short record was read")
	}
	if err := back.UnmarshalBinary([]byte("JWH1")); err == nil {
		t.Error("another record's form was read")
	}
}

// With the default memory of 26 departures each record keeps 1 - 2/27 of
// its weight at the next. A cabin of 100 bookings where 10 stayed away
// for a long time, and then 30 do: after 1 such departure the mean is
// (0.9259*W*0.90 + 70)/(0.9259*W + 100) with W the settled weight
// 100/(2/27) = 1350, which is (1125 + 70)/(1250 + 100) = 0.8852.
func TestShowsForgets(t *testing.T) {
	s := &Shows{}
	for i := 0; i < 400; i++ {
		s.Record(100, 10)
	}
	if mean, dev, _ := s.Rate(); math.Abs(mean-0.90) > 1e-9 || dev > 1e-6 {
		t.Fatalf("settled: %.5f, %.6f; want 0.90 and 0", mean, dev)
	}
	s.Record(100, 30)
	if mean, _, _ := s.Rate(); math.Abs(mean-0.8852) > 0.0001 {
		t.Fatalf("after 1 departure at 0.70: %.5f, want 0.8852", mean)
	}
	for i := 0; i < 400; i++ {
		s.Record(100, 30)
	}
	if mean, _, _ := s.Rate(); math.Abs(mean-0.70) > 1e-6 {
		t.Fatalf("after 400 departures at 0.70: %.5f, want 0.70", mean)
	}
}

// The lecture's relationships, on a cabin of 100 seats authorised to
// 110. "PAX = min [ BKD - NS + GS + SB, CAP]" and "SP = CAP - PAX, only
// when BKD = AU".
func TestDepartureKeepsUnsoldNoShowAndSpoiledApart(t *testing.T) {
	for _, c := range []struct {
		name                             string
		d                                Departure
		showUps, boarded, denied, spoilt int
	}{
		// 90 booked and all came: 10 seats empty, all unsold.
		{"unsold", Departure{Capacity: 100, Authorized: 110, Booked: 90}, 90, 90, 0, 0},
		// 100 booked, 8 stayed away: 8 seats empty, and the cabin never
		// reached its limit of 110, so none is spoiled.
		{"no-shows under the limit", Departure{Capacity: 100, Authorized: 110, Booked: 100, NoShows: 8}, 92, 92, 0, 0},
		// 110 booked, 15 stayed away: 95 came, 5 seats spoiled.
		{"spoiled", Departure{Capacity: 100, Authorized: 110, Booked: 110, NoShows: 15}, 95, 95, 0, 5},
		// 110 booked, 6 stayed away: 104 came for 100 seats.
		{"denied", Departure{Capacity: 100, Authorized: 110, Booked: 110, NoShows: 6}, 104, 100, 4, 0},
		// 2 standbys take 2 of the 5 seats the no-shows left.
		{"standbys", Departure{Capacity: 100, Authorized: 110, Booked: 110, NoShows: 15, Standbys: 2}, 95, 97, 0, 3},
		// A cabin that is not overbooked spoils every no-show's seat once
		// it is full.
		{"not overbooked", Departure{Capacity: 100, Booked: 100, NoShows: 8}, 92, 92, 0, 8},
	} {
		if got := c.d.ShowUps(); got != c.showUps {
			t.Errorf("%s: show-ups %d, want %d", c.name, got, c.showUps)
		}
		if got := c.d.Boarded(); got != c.boarded {
			t.Errorf("%s: boarded %d, want %d", c.name, got, c.boarded)
		}
		if got := c.d.Denied(); got != c.denied {
			t.Errorf("%s: denied %d, want %d", c.name, got, c.denied)
		}
		if got := c.d.Spoiled(); got != c.spoilt {
			t.Errorf("%s: spoiled %d, want %d", c.name, got, c.spoilt)
		}
	}
}

// A cabin of 174 seats authorised to 180 confirms 180 and waitlists the
// next request. Availability counts the seats left to the limit. The
// front cabin of another flight is not authorised over its 8 seats and
// sells 8.
func TestInventorySellsToTheAuthorisedCapacity(t *testing.T) {
	inv := New("WN", b737())
	asked := 0
	inv.Authorized = func(carrier, flight, date, board, comp string, seats int) int {
		asked++
		if carrier != "WN" || date != "26NOV" || board != "BNA" {
			t.Errorf("asked for %s %s %s %s", carrier, flight, date, board)
		}
		if flight == "2554" && comp == "Y" {
			if seats != 174 {
				t.Errorf("seats = %d, want 174", seats)
			}
			return 180
		}
		return seats
	}
	day := time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC)
	left := func() avail.Entry {
		av := inv.Availability([]avail.Key{avail.NewKey("WN", "2554", day, "BNA", "DCA", "Y")}, time.Now())
		if len(av) != 1 {
			t.Fatalf("availability: %d entries", len(av))
		}
		return av[0]
	}
	if e := left(); e.Seats != 180 || e.Status != avail.Open {
		t.Fatalf("before any sale: %d seats, %v; want 180 open", e.Seats, e.Status)
	}
	for i := 0; i < 87; i++ {
		if got := decide(t, inv, seg("2554", "Y", "HN", 2)); got != "KK" {
			t.Fatalf("sell %d: %s", i, got)
		}
	}
	// 174 sold: the seats are gone and 6 bookings are left.
	if e := left(); e.Seats != 6 || e.Status != avail.Open {
		t.Fatalf("at the seat count: %d seats, %v; want 6 open", e.Seats, e.Status)
	}
	if got := decide(t, inv, seg("2554", "Y", "HN", 4)); got != "KK" {
		t.Fatalf("seats 175 to 178: %s", got)
	}
	if got := decide(t, inv, seg("2554", "Y", "HN", 4)); got != "US" {
		t.Fatalf("4 more would pass the limit of 180: %s", got)
	}
	if got := decide(t, inv, seg("2554", "Y", "HN", 2)); got != "KK" {
		t.Fatalf("seats 179 and 180: %s", got)
	}
	if got := decide(t, inv, seg("2554", "Y", "HN", 1)); got == "KK" {
		t.Fatal("a booking over the limit confirmed")
	}
	if e := left(); e.Seats != 0 || e.Status == avail.Open {
		t.Fatalf("at the limit: %d seats, %v", e.Seats, e.Status)
	}
	for _, p := range inv.Snapshot() {
		if p.Flight == "WN2554" && (p.Sold != 180 || p.Seats != 174 || p.Authorized != 180) {
			t.Errorf("snapshot: %+v; want 180 sold of 174 seats, 180 authorised", p)
		}
	}
	// A cancelled booking gives its seats back under the limit.
	inv.Release(context.Background(), seg("2554", "Y", "HK", 2), "HK")
	if e := left(); e.Seats != 2 || e.Status != avail.Open {
		t.Fatalf("after a cancellation: %d seats, %v; want 2 open", e.Seats, e.Status)
	}

	for i := 0; i < 8; i++ {
		if got := decide(t, inv, seg("1", "F", "HN", 1)); got != "KK" {
			t.Fatalf("first class seat %d: %s", i+1, got)
		}
	}
	if got := decide(t, inv, seg("1", "F", "HN", 1)); got == "KK" {
		t.Fatal("a ninth first class seat confirmed")
	}
	if asked == 0 {
		t.Fatal("revenue management was never asked")
	}

	// An authorised capacity under the seats holds seats back.
	held := New("WN", b737())
	held.Authorized = func(_, _, _, _, _ string, seats int) int { return 2 }
	if got := decide(t, held, seg("2554", "Y", "HN", 2)); got != "KK" {
		t.Fatalf("2 of 2: %s", got)
	}
	if got := decide(t, held, seg("2554", "Y", "HN", 1)); got == "KK" {
		t.Fatal("a third booking confirmed on a cabin held to 2")
	}
}

// A controller told the cabin's authorised capacity nests its ladder
// under that. 174 seats authorised to 180: the highest class may sell
// 180, and the forecaster is still told 174 seats.
func TestControllerNestsUnderTheAuthorisedCapacity(t *testing.T) {
	told := 0
	c := &Controller{Capacity: b737(), Forecast: func(_, _, _, _, _ string, seats int) []ClassDemand {
		told = seats
		return []ClassDemand{{Class: "Y", Fare: 300, Mean: 20, StdDev: 5}, {Class: "Q", Fare: 100, Mean: 200, StdDev: 20}}
	}}
	plain := c.Levels("WN", "2554", "26NOV", "BNA", "Y")
	c.Authorized = func(_, _, _, _, comp string, seats int) int { return seats + 6 }
	over := c.Levels("WN", "2554", "26NOV", "BNA", "Y")
	if told != 174 {
		t.Errorf("the forecaster was told %d seats, want 174", told)
	}
	if len(plain) != 2 || len(over) != 2 || plain[0].Authorized != 174 || over[0].Authorized != 180 {
		t.Fatalf("ladders %+v and %+v; want the highest class at 174 and at 180", plain, over)
	}
	// The seats held back for the higher class do not change, so the
	// lower class has the 6 more.
	if over[1].Authorized != plain[1].Authorized+6 {
		t.Errorf("the lower class may sell %d and %d; want 6 more", plain[1].Authorized, over[1].Authorized)
	}
	if l, _ := c.LadderFares("WN", "2554", "26NOV", "BNA", "Y"); len(l) != 2 || l[0].Authorized != 180 {
		t.Errorf("the priced ladder: %+v; want the highest class at 180", l)
	}
}
