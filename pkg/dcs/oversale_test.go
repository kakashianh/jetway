package dcs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adamf/jetway/pkg/pnl"
)

// small is a departure control whose aircraft has 1 seat in business
// (1A) and 4 in economy (2A, 2C, 3A, 3C).
func small(t *testing.T) *Station {
	t.Helper()
	s := station(t, "SMALL")
	s.Fleet = &FleetData{DefaultType: "SMALL", Types: map[string]*AircraftType{"SMALL": {
		Code: "SMALL", DOW: 1000, MZFW: 5000, MTOW: 6000, MLW: 5500, RefArm: 5, C: 100, K: 50, LEMAC: 4, MAC: 2, FwdMAC: 10, AftMAC: 90, FuelArm: 5,
		Cabin: CabinLayout{Sections: []Section{
			{Compartment: "C", FromRow: 1, ToRow: 1, Letters: "A"},
			{Compartment: "Y", FromRow: 2, ToRow: 3, Letters: "AC"},
		}},
		Compartments: []Compartment{{Name: "1", Max: 500, Arm: 4}},
		Zones:        []Zone{{Name: "OA", FromRow: 1, ToRow: 3, Arm: 5}},
	}}}
	s.Equipment = func(k Key) (Equipment, bool) { return Equipment{Type: "SMALL", Dest: "JFK"}, true }
	return s
}

func named(f *Flight, surname string) *Passenger {
	for _, p := range f.Passengers {
		if p.Surname == surname {
			return p
		}
	}
	return nil
}

// The economy cabin has 4 seats and 8 confirmed bookings. The business
// cabin has 1 seat and 1 booking.
//
//   - ALFA, BRAVO, CHARLIE and DELTA come first and take the 4 seats.
//   - ECHO, HOTEL and INDIA come to a full cabin, in that order. Each
//     waits for a seat with a claim to one. ECHO brings a bag of 20 kg.
//   - FOXTROT in economy and FRONT in business never come.
//   - GOLF comes with a ticket and no booking, and stands by.
//
// When check-in closes, FRONT's seat 1A is free. ECHO has waited longest
// and takes it, in the other cabin, and the bag is tagged then. HOTEL and
// INDIA still wait, and GOLF is behind them. BRAVO volunteers: BRAVO's
// seat goes to HOTEL. At the door INDIA has no seat and is denied
// boarding.
//
// The count for economy: 8 booked, 1 no-show (FOXTROT), 7 came. Of the 7,
// 5 flew (ALFA, CHARLIE, DELTA, ECHO, HOTEL) and 2 were denied boarding
// (BRAVO by agreement, INDIA not). No seat left empty. The count for
// business: 1 booked, 1 no-show, and the seat flew with an economy
// passenger in it.
func TestOversoldCabinIsResolvedInOrder(t *testing.T) {
	ctx := context.Background()
	s := small(t)
	m, err := pnl.Parse(strings.Join([]string{
		"PNL",
		"BA0117/16DEC LHR PART1",
		"-JFK001C",
		"1FRONT/FAYMS .L/FFF000",
		"-JFK008Y",
		"1ALFA/ANNMS .L/AAA001",
		"1BRAVO/BOBMR .L/BBB002",
		"1CHARLIE/CYMR .L/CCC003",
		"1DELTA/DEEMS .L/DDD004",
		"1ECHO/EDMR .L/EEE005",
		"1FOXTROT/FLOMS .L/FFF006",
		"1HOTEL/HALMR .L/HHH008",
		"1INDIA/IDAMS .L/III009",
		"ENDPNL",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyNameList(ctx, m); err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{"AAA001", "BBB002", "CCC003", "DDD004"} {
		acc, err := s.Accept(ctx, testKey, AcceptRequest{Locator: loc})
		if err != nil || acc.Passengers[0].Status != StatusAccepted || acc.Passengers[0].Seat == "" {
			t.Fatalf("%s: %v %+v", loc, err, acc)
		}
	}

	// A booked passenger at a full cabin waits. Nothing is refused.
	acc, err := s.Accept(ctx, testKey, AcceptRequest{Locator: "EEE005", Bags: []int{20}})
	if err != nil {
		t.Fatalf("ECHO at a full cabin was refused: %v", err)
	}
	echo := acc.Passengers[0]
	if echo.Status != StatusStandby || echo.Oversale != OversaleSold || echo.Seat != "" || !echo.Claimant() {
		t.Fatalf("ECHO: %+v; want a standby with a claim and no seat", echo)
	}
	if len(acc.Tags) != 0 || len(echo.Bags) != 0 || len(echo.HeldBags) != 1 || echo.HeldBags[0] != 20 {
		t.Fatalf("ECHO's bag: tags %v, bags %v, held %v; want held and not tagged", acc.Tags, echo.Bags, echo.HeldBags)
	}
	for _, loc := range []string{"HHH008", "III009"} {
		if acc, err := s.Accept(ctx, testKey, AcceptRequest{Locator: loc}); err != nil || !acc.Passengers[0].Claimant() {
			t.Fatalf("%s: %v %+v", loc, err, acc)
		}
	}
	if _, err := s.AcceptGoShow(ctx, testKey, GoShow{Surname: "GOLF", Given: "GUSMR"}); err != nil {
		t.Fatal(err)
	}
	waiting, err := s.Waiting(testKey)
	if err != nil || len(waiting) != 3 || waiting[0].Surname != "ECHO" || waiting[1].Surname != "HOTEL" || waiting[2].Surname != "INDIA" {
		t.Fatalf("waiting: %v %+v", err, waiting)
	}
	// A volunteer's seat is not asked for while the business seat may
	// still be claimed by its own passenger: ECHO does not take 1A yet.
	if p := named(mustFlight(t, s), "ECHO"); p.Seat != "" {
		t.Fatalf("ECHO was seated in %s while check-in was open", p.Seat)
	}

	f, cleared, err := s.CloseCheckIn(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(cleared) != 1 || cleared[0].Surname != "ECHO" {
		t.Fatalf("cleared at check-in close: %+v; want ECHO alone", cleared)
	}
	echo = named(f, "ECHO")
	if echo.Status != StatusAccepted || echo.Seat != "1A" || echo.Compartment != "C" || echo.Oversale != "" {
		t.Fatalf("ECHO after check-in close: %+v; want accepted in 1A", echo)
	}
	if len(echo.Bags) != 1 || echo.Bags[0].Weight != 20 || len(echo.HeldBags) != 0 {
		t.Fatalf("ECHO's bag after seating: %+v held %v", echo.Bags, echo.HeldBags)
	}
	if g := named(f, "GOLF"); g.Status != StatusStandby {
		t.Fatalf("GOLF has no booking and was put ahead of passengers who have: %+v", g)
	}
	if c := f.Counts(); c.Waiting != 2 || c.Standby != 3 {
		t.Fatalf("after check-in close: %d wait with a claim of %d standbys; want 2 of 3", c.Waiting, c.Standby)
	}
	wanted := false
	for _, a := range f.Alerts {
		if a.Code == "oversold" && strings.Contains(a.Detail, "volunteers") {
			wanted = true
		}
	}
	if !wanted {
		t.Error("no alert asked for volunteers")
	}

	// A late passenger has no claim: FOXTROT, forced in after check-in
	// closed, finds no seat and does not join those who wait.
	if _, err := s.Accept(ctx, testKey, AcceptRequest{Locator: "FFF006", Force: true}); !errors.Is(err, ErrNoSeat) {
		t.Fatalf("FOXTROT after check-in close: %v, want no seat", err)
	}

	v, err := s.Volunteer(ctx, testKey, named(f, "BRAVO").ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Volunteer.Status != StatusDenied || !v.Volunteer.Volunteer {
		t.Fatalf("BRAVO: %+v; want denied boarding by agreement", v.Volunteer)
	}
	if v.Seated == nil || v.Seated.Surname != "HOTEL" || v.Seated.Seat != v.Volunteer.Seat || v.Seated.Status != StatusAccepted {
		t.Fatalf("BRAVO's seat %s went to %+v; want HOTEL", v.Volunteer.Seat, v.Seated)
	}

	f = mustFlight(t, s)
	for _, name := range []string{"ALFA", "CHARLIE", "DELTA", "ECHO", "HOTEL"} {
		if _, err := s.Board(ctx, testKey, named(f, name).ID); err != nil {
			t.Fatalf("board %s: %v", name, err)
		}
	}
	cl, err := s.CloseFlight(ctx, testKey, CloseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.Denied) != 2 {
		t.Fatalf("denied at close: %+v; want BRAVO and INDIA", cl.Denied)
	}
	india := named(cl.Flight, "INDIA")
	if india.Status != StatusDenied || india.Volunteer || india.Oversale != OversaleSold || india.Category != CategoryInvol {
		t.Fatalf("INDIA: %+v; want denied boarding, not by agreement", india)
	}
	if p := named(cl.Flight, "FOXTROT"); p.Status != StatusNoShow {
		t.Fatalf("FOXTROT: %s, want a no-show", p.Status)
	}
	c := cl.Counts
	if c.Boarded != 5 || c.NoShow != 2 || c.Denied != 2 || c.Volunteers != 1 || c.Waiting != 0 {
		t.Fatalf("counts: %+v; want 5 boarded, 2 no-shows, 2 denied of whom 1 volunteer", c)
	}

	// The final sales message keeps them apart: who did not come, who
	// came off by agreement, and who was turned away.
	pfs := strings.Join(cl.PFS, "\n")
	section := func(category string) string {
		_, after, ok := strings.Cut(pfs, "\n"+category+"\n")
		if !ok {
			return ""
		}
		for _, other := range pfsCategories {
			if before, _, cut := strings.Cut(after, "\n"+other+"\n"); cut {
				after = before
			}
		}
		if before, _, cut := strings.Cut(after, "\n-"); cut {
			after = before
		}
		return after
	}
	for _, want := range []struct{ category, in, out string }{
		{"NOSHO", "FOXTROT", "INDIA"},
		{"NOSHO", "FRONT", "BRAVO"},
		{"OFFLD", "BRAVO", "INDIA"},
		{"INVOL", "INDIA", "FOXTROT"},
	} {
		got := section(want.category)
		if !strings.Contains(got, want.in) || strings.Contains(got, want.out) {
			t.Errorf("PFS %s is %q; want %s in it and %s not\n%s", want.category, got, want.in, want.out, pfs)
		}
	}
	if strings.Contains(pfs, "GOLF") {
		t.Errorf("GOLF did not fly and had no booking, and is in the PFS:\n%s", pfs)
	}

	tally := cl.Flight.Tally()
	if got, want := tally["Y"], (Tally{Seats: 4, Booked: 8, NoShows: 1, Denied: 2, Volunteers: 1, Boarded: 5}); got != want {
		t.Errorf("economy: %+v, want %+v", got, want)
	}
	if got, want := tally["C"], (Tally{Seats: 1, Booked: 1, NoShows: 1}); got != want {
		t.Errorf("business: %+v, want %+v", got, want)
	}
	if y := tally["Y"]; y.ShowUps() != 7 || y.ShowUps() != y.Boarded+y.Denied+y.Offloaded {
		t.Errorf("economy: %d came, and %d flew, %d were denied, %d came off", y.ShowUps(), y.Boarded, y.Denied, y.Offloaded)
	}
}

// 4 economy seats and 5 bookings. 4 are accepted and the fifth waits.
// Nobody volunteers. One accepted passenger never comes to the gate. At
// the door that seat goes to the passenger who waits, who flies. Nobody
// is denied boarding, and the one who stayed in the lounge is an offload,
// not a no-show.
func TestAGateNoShowsSeatGoesToThePassengerWhoWaits(t *testing.T) {
	ctx := context.Background()
	s := small(t)
	m, err := pnl.Parse(strings.Join([]string{
		"PNL", "BA0117/16DEC LHR PART1", "-JFK005Y",
		"1ALFA/ANNMS .L/AAA001", "1BRAVO/BOBMR .L/BBB002", "1CHARLIE/CYMR .L/CCC003", "1DELTA/DEEMS .L/DDD004", "1ECHO/EDMR .L/EEE005",
		"ENDPNL",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyNameList(ctx, m); err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{"AAA001", "BBB002", "CCC003", "DDD004", "EEE005"} {
		if _, err := s.Accept(ctx, testKey, AcceptRequest{Locator: loc}); err != nil {
			t.Fatalf("%s: %v", loc, err)
		}
	}
	// Nobody waits in business, so a volunteer is not taken there, and
	// one is not taken from a flight where nobody waits at all.
	f, cleared, err := s.CloseCheckIn(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	// The business seat is free and nobody is booked in it: ECHO takes it.
	if len(cleared) != 1 || cleared[0].Surname != "ECHO" || cleared[0].Seat != "1A" {
		t.Fatalf("cleared: %+v; want ECHO in 1A", cleared)
	}
	if _, err := s.Volunteer(ctx, testKey, named(f, "ALFA").ID); !errors.Is(err, ErrNotOversold) {
		t.Fatalf("a volunteer on a flight where nobody waits: %v", err)
	}

	// The same flight with the business seat taken.
	s = small(t)
	m, _ = pnl.Parse(strings.Join([]string{
		"PNL", "BA0117/16DEC LHR PART1", "-JFK001C", "1FRONT/FAYMS .L/FFF000", "-JFK005Y",
		"1ALFA/ANNMS .L/AAA001", "1BRAVO/BOBMR .L/BBB002", "1CHARLIE/CYMR .L/CCC003", "1DELTA/DEEMS .L/DDD004", "1ECHO/EDMR .L/EEE005",
		"ENDPNL",
	}, "\n"))
	if _, err := s.ApplyNameList(ctx, m); err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{"FFF000", "AAA001", "BBB002", "CCC003", "DDD004", "EEE005"} {
		if _, err := s.Accept(ctx, testKey, AcceptRequest{Locator: loc}); err != nil {
			t.Fatalf("%s: %v", loc, err)
		}
	}
	f, cleared, err = s.CloseCheckIn(ctx, testKey)
	if err != nil || len(cleared) != 0 {
		t.Fatalf("check-in close: %v, cleared %+v; want nobody", err, cleared)
	}
	for _, name := range []string{"FRONT", "ALFA", "BRAVO", "CHARLIE"} {
		if _, err := s.Board(ctx, testKey, named(f, name).ID); err != nil {
			t.Fatal(err)
		}
	}
	cl, err := s.CloseFlight(ctx, testKey, CloseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.Onloaded) != 1 || cl.Onloaded[0].Surname != "ECHO" || cl.Onloaded[0].Status != StatusBoarded {
		t.Fatalf("onloaded at the door: %+v; want ECHO, boarded", cl.Onloaded)
	}
	if len(cl.Denied) != 0 {
		t.Fatalf("denied: %+v; want nobody", cl.Denied)
	}
	if d := named(cl.Flight, "DELTA"); d.Status != StatusOffloaded {
		t.Fatalf("DELTA: %s, want offloaded", d.Status)
	}
	if got, want := cl.Flight.Tally()["Y"], (Tally{Seats: 4, Booked: 5, Offloaded: 1, Boarded: 4}); got != want {
		t.Errorf("economy: %+v, want %+v", got, want)
	}
}

// A smaller aircraft takes a flight after 5 passengers were accepted on
// one with more seats. The one it has no seat for waits with a claim, and
// is denied boarding when the flight closes. The cause is kept, because
// the law treats a smaller aircraft and an oversold cabin differently.
func TestAPassengerDisplacedByASmallerAircraftIsDeniedNotLost(t *testing.T) {
	ctx := context.Background()
	s := small(t)
	big := *s.Fleet.Types["SMALL"]
	big.Code = "BIG"
	big.Cabin = CabinLayout{Sections: []Section{{Compartment: "Y", FromRow: 1, ToRow: 3, Letters: "AC"}}}
	s.Fleet.Types["BIG"] = &big
	s.Equipment = func(k Key) (Equipment, bool) { return Equipment{Type: "BIG", Dest: "JFK"}, true }
	m, err := pnl.Parse(strings.Join([]string{
		"PNL", "BA0117/16DEC LHR PART1", "-JFK006Y",
		"1ALFA/ANNMS .L/AAA001", "1BRAVO/BOBMR .L/BBB002", "1CHARLIE/CYMR .L/CCC003", "1DELTA/DEEMS .L/DDD004", "1ECHO/EDMR .L/EEE005", "1FOXTROT/FLOMS .L/FFF006",
		"ENDPNL",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyNameList(ctx, m); err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{"AAA001", "BBB002", "CCC003", "DDD004", "EEE005", "FFF006"} {
		if _, err := s.Accept(ctx, testKey, AcceptRequest{Locator: loc}); err != nil {
			t.Fatalf("%s: %v", loc, err)
		}
	}
	// 6 accepted, and the smaller aircraft has 5 seats in all.
	ch, err := s.ChangeEquipment(ctx, testKey, Equipment{Type: "SMALL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ch.Displaced) != 1 {
		t.Fatalf("displaced: %+v; want 1", ch.Displaced)
	}
	who := ch.Displaced[0].Surname
	if p := named(ch.Flight, who); !p.Claimant() || p.Oversale != OversaleEquipment {
		t.Fatalf("%s: %+v; want waiting with a claim, cause the aircraft", who, p)
	}
	f, _, err := s.CloseCheckIn(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range f.Passengers {
		if p.Status == StatusAccepted {
			if _, err := s.Board(ctx, testKey, p.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	cl, err := s.CloseFlight(ctx, testKey, CloseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cl.Denied) != 1 || cl.Denied[0].Surname != who || cl.Denied[0].Oversale != OversaleEquipment || cl.Denied[0].Volunteer {
		t.Fatalf("denied: %+v; want %s, cause the aircraft", cl.Denied, who)
	}
	if c := cl.Counts; c.NoShow != 0 || c.Denied != 1 || c.Boarded != 5 {
		t.Fatalf("counts: %+v", c)
	}
}

// While check-in is open, the business seat belongs to the passenger
// booked in it, who has not come yet. 4 economy seats are taken and ECHO
// and HOTEL wait. CHARLIE volunteers early: CHARLIE's seat goes to ECHO.
// HOTEL is not put in the business seat, because FRONT may still come.
// FRONT does come and has the seat. HOTEL is denied boarding at the door.
func TestASeatOwedToABookedPassengerIsNotGivenAwayWhileCheckInIsOpen(t *testing.T) {
	ctx := context.Background()
	s := small(t)
	m, err := pnl.Parse(strings.Join([]string{
		"PNL", "BA0117/16DEC LHR PART1", "-JFK001C", "1FRONT/FAYMS .L/FFF000", "-JFK006Y",
		"1ALFA/ANNMS .L/AAA001", "1BRAVO/BOBMR .L/BBB002", "1CHARLIE/CYMR .L/CCC003", "1DELTA/DEEMS .L/DDD004", "1ECHO/EDMR .L/EEE005", "1HOTEL/HALMR .L/HHH008",
		"ENDPNL",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyNameList(ctx, m); err != nil {
		t.Fatal(err)
	}
	for _, loc := range []string{"AAA001", "BBB002", "CCC003", "DDD004", "EEE005", "HHH008"} {
		if _, err := s.Accept(ctx, testKey, AcceptRequest{Locator: loc}); err != nil {
			t.Fatalf("%s: %v", loc, err)
		}
	}
	f := mustFlight(t, s)
	v, err := s.Volunteer(ctx, testKey, named(f, "CHARLIE").ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Seated == nil || v.Seated.Surname != "ECHO" || v.Seated.Compartment != "Y" {
		t.Fatalf("CHARLIE's seat went to %+v; want ECHO, in economy", v.Seated)
	}
	if h := named(v.Flight, "HOTEL"); !h.Claimant() || h.Seat != "" {
		t.Fatalf("HOTEL: %+v; want still waiting, and not in the seat FRONT is booked in", h)
	}
	acc, err := s.Accept(ctx, testKey, AcceptRequest{Locator: "FFF000"})
	if err != nil || acc.Passengers[0].Seat != "1A" {
		t.Fatalf("FRONT: %v %+v; want seat 1A", err, acc)
	}
	f, _, err = s.CloseCheckIn(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"FRONT", "ALFA", "BRAVO", "DELTA", "ECHO"} {
		if _, err := s.Board(ctx, testKey, named(f, name).ID); err != nil {
			t.Fatalf("board %s: %v", name, err)
		}
	}
	cl, err := s.CloseFlight(ctx, testKey, CloseOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range cl.Denied {
		names = append(names, p.Surname)
	}
	if len(cl.Onloaded) != 0 || strings.Join(names, " ") != "CHARLIE HOTEL" {
		t.Fatalf("onloaded %+v, denied %v; want nobody onloaded, CHARLIE and HOTEL denied", cl.Onloaded, names)
	}
	if h := named(cl.Flight, "HOTEL"); h.Volunteer || h.Status != StatusDenied {
		t.Fatalf("HOTEL: %+v; want denied boarding, not by agreement", h)
	}
}
