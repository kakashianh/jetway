package dcs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// An oversold cabin.
//
// A carrier that overbooks sells a cabin more confirmed bookings than it
// has seats, against the passengers who will not come (pkg/inventory,
// overbook.go). On most days enough stay away. On the others more
// passengers with confirmed bookings come than the cabin seats, and the
// airport resolves it in the order below. A smaller aircraft taking the
// flight puts passengers in the same position.
//
//  1. While check-in is open, a booked passenger who comes to a full
//     cabin is not refused. They wait for a seat with a claim to one: a
//     standby ahead of every other, with their bags held.
//  2. When check-in closes, the passengers on the list who have not come
//     have lost their seats. Those who wait take the free seats, in
//     their own cabin first and in another cabin when it is full.
//  3. If some still wait, the agent asks seated passengers to give up
//     their seats for the carrier's offer. Each volunteer's seat goes to
//     the passenger who has waited longest. A volunteer is a denied
//     boarding the passenger agreed to.
//  4. When the door closes, a seat an accepted passenger never came to
//     the gate for goes to a passenger who waits. One who still has no
//     seat is denied boarding against their will.
//
// A passenger denied boarding came to the airport. They are never
// reported as a no-show, and the count a revenue management system takes
// from the closed flight (Tally) keeps the two apart.
//
// An infant on a lap holds no seat, so it has no claim to one and none to
// give up. It waits with its adults, is accepted with the first of them
// who is seated, and comes off with the last of them who does not fly. It
// is not a volunteer and not a denied boarding, and it is in none of the
// counts of seats: not in Tally, and not in the waiting, denied and
// volunteer counts of Counts.

// ErrNotOversold answers a volunteer when nobody waits for a seat.
var ErrNotOversold = errors.New("dcs: no passenger with a claim waits for a seat")

// ErrLapInfant answers a volunteer who is an infant on a lap.
var ErrLapInfant = errors.New("dcs: an infant on a lap holds no seat to give up")

// confirmed reports whether every passenger of a group holds a confirmed
// booking and has not been handled yet: the name list carried them.
func confirmed(group []*Passenger) bool {
	for _, p := range group {
		if p.Status != StatusListed || p.Category != CategoryListed {
			return false
		}
	}
	return len(group) > 0
}

// wait puts a passenger on standby with a claim to a seat.
func (f *Flight) wait(p *Passenger, why Oversale, now time.Time) {
	p.Status = StatusStandby
	p.Oversale = why
	t := now
	p.WaitingSince = &t
}

// waiting counts the passengers who wait for a seat with a claim to one.
// An infant who waits with its adults needs no seat.
func (f *Flight) waiting() int {
	n := 0
	for _, p := range f.Passengers {
		if p.Claimant() && p.Type != PaxInfant {
			n++
		}
	}
	return n
}

// adultFlies reports whether an adult of a party holds a seat on the
// flight: someone whose lap an infant of the party can be on.
func (f *Flight) adultFlies(party string) bool {
	for _, p := range f.Passengers {
		if p.Party == party && p.Type == PaxAdult && p.Flying() {
			return true
		}
	}
	return false
}

// claimants lists the passengers who wait with a claim, the one who has
// waited longest first.
func (f *Flight) claimants() []*Passenger {
	var out []*Passenger
	for _, p := range f.Passengers {
		if p.Claimant() {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].WaitingSince, out[j].WaitingSince
		if a != nil && b != nil && !a.Equal(*b) {
			return a.Before(*b)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// seatClaimants seats the passengers who wait with a claim in whatever
// seats are free, in their own cabin first and in any other when it is
// full, and returns those it seated.
func (s *Station) seatClaimants(f *Flight) []*Passenger {
	var seated []*Passenger
	// An infant who waits goes with the adult: when a passenger of the
	// party is seated, the party's infants are accepted with them.
	withInfants := func(p *Passenger) {
		for _, q := range f.Passengers {
			if q.Party == p.Party && q.Type == PaxInfant && q.Claimant() {
				if _, err := s.accept(f, []*Passenger{q}, AcceptRequest{}); err == nil {
					seated = append(seated, q)
				}
			}
		}
	}
	for _, p := range f.claimants() {
		if p.Type == PaxInfant {
			continue
		}
		if _, err := s.accept(f, []*Passenger{p}, AcceptRequest{}); err == nil {
			seated = append(seated, p)
			withInfants(p)
			continue
		}
		for _, comp := range f.Cabin.compartments() {
			// While check-in is open a seat in another cabin may be owed
			// to a passenger who has not come yet.
			if comp == p.Compartment || (f.State == StateOpen && f.spare(comp) <= 0) {
				continue
			}
			seats, err := f.Cabin.Assign(comp, 1)
			if err != nil || len(seats) == 0 {
				continue
			}
			// accept records the move to another cabin on the flight.
			if _, err := s.accept(f, []*Passenger{p}, AcceptRequest{Seat: seats[0]}); err == nil {
				seated = append(seated, p)
				withInfants(p)
				break
			}
		}
	}
	return seated
}

// Waiting lists the passengers who wait for a seat with a claim to one,
// the one who has waited longest first.
func (s *Station) Waiting(k Key) ([]*Passenger, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flights[k]
	if !ok {
		return nil, ErrFlightNotFound
	}
	var out []*Passenger
	for _, p := range f.claimants() {
		c := *p
		out = append(out, &c)
	}
	return out, nil
}

// Volunteered is what a volunteer's seat did.
type Volunteered struct {
	Flight *Flight `json:"flight"`
	// Volunteer is the passenger who gave up the seat, with their bags
	// still attached so the caller can pull them.
	Volunteer *Passenger `json:"volunteer"`
	// Seated is the passenger who took a seat because of it, with the
	// tags of the bags they had brought. Nil when the seat suited nobody
	// who waits.
	Seated *Passenger `json:"seated,omitempty"`
	// With is the infants who came off with the volunteer: those on the
	// laps of a party that has no other adult on the flight. They are not
	// volunteers. Their bags are still attached, for the caller to pull.
	With []*Passenger `json:"with,omitempty"`
}

// Volunteer records that a passenger who holds a seat gives it up for the
// carrier's offer, on a flight where passengers wait for seats: a denied
// boarding the passenger agreed to. The seat goes to the passenger who
// has waited longest. What the carrier offered is the caller's to record
// and to pay.
func (s *Station) Volunteer(ctx context.Context, k Key, passengerID int) (*Volunteered, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flights[k]
	if !ok {
		return nil, ErrFlightNotFound
	}
	if f.State == StateClosed {
		return nil, ErrFlightClosed
	}
	p := f.passenger(passengerID)
	if p == nil {
		return nil, ErrPassengerNotFound
	}
	if !p.Flying() {
		return nil, ErrNotAccepted
	}
	if p.Type == PaxInfant {
		return nil, ErrLapInfant
	}
	if f.waiting() == 0 {
		return nil, ErrNotOversold
	}
	now := s.now()
	if p.Seat != "" {
		f.Cabin.Release(p.Seat)
	}
	p.Status = StatusDenied
	p.Volunteer = true
	p.OffloadReason = "volunteer"
	for i := range p.Bags {
		p.Bags[i].Offloaded = true
	}
	out := &Volunteered{}
	// An infant on the volunteer's lap comes off with them, unless
	// another adult of the party still flies.
	if !f.adultFlies(p.Party) {
		for _, q := range f.Passengers {
			if q.Party != p.Party || q.Type != PaxInfant || !q.Flying() {
				continue
			}
			q.Status = StatusDenied
			q.Volunteer = true
			q.OffloadReason = "with volunteer"
			for i := range q.Bags {
				q.Bags[i].Offloaded = true
			}
			c := *q
			out.With = append(out.With, &c)
		}
	}
	if seated := s.seatClaimants(f); len(seated) > 0 {
		c := *seated[0]
		out.Seated = &c
	}
	f.alert(now, "volunteer", fmt.Sprintf("%s gave up %s", p.Display(), p.Seat))
	f.Revision++
	if err := s.store().SaveFlight(ctx, f); err != nil {
		return nil, err
	}
	v := *p
	out.Volunteer, out.Flight = &v, f.snapshot()
	return out, nil
}

// Tally is one cabin of a flight counted as revenue management counts
// it. Passengers are counted in the cabin they booked, whichever cabin
// seated them. An infant on a lap is in none of the counts: it is not a
// booking of a seat.
type Tally struct {
	// Seats is the cabin's seats on the aircraft that took the flight.
	Seats int `json:"seats"`
	// Booked is the passengers the name list carried: the confirmed
	// bookings at departure.
	Booked int `json:"booked"`
	// NoShows is the booked passengers who never came to check in.
	NoShows int `json:"no_shows"`
	// Offloaded is the booked passengers who were accepted and did not
	// fly, other than by being denied boarding: one who never came to the
	// gate, or one an agent took off.
	Offloaded int `json:"offloaded"`
	// Denied is the booked passengers who came and were denied boarding,
	// and Volunteers those of them who agreed.
	Denied     int `json:"denied"`
	Volunteers int `json:"volunteers"`
	// Boarded is the booked passengers who flew.
	Boarded int `json:"boarded"`
	// Others is the passengers who flew without a booking on the list:
	// go-shows, no-records and staff.
	Others int `json:"others"`
}

// ShowUps is the booked passengers who came for the flight.
func (t Tally) ShowUps() int { return t.Booked - t.NoShows }

// Empty is the seats that left empty.
func (t Tally) Empty() int { return max(t.Seats-t.Boarded-t.Others, 0) }

// Tally counts a flight's cabins. It is what a closed flight tells
// revenue management about who came. An empty seat with no booking
// behind it is in neither NoShows nor Denied: it was never sold.
func (f *Flight) Tally() map[string]Tally {
	out := map[string]Tally{}
	if f.Cabin != nil {
		for _, comp := range f.Cabin.compartments() {
			out[comp] = Tally{Seats: f.Cabin.Free(comp) + f.occupied(comp)}
		}
	}
	for _, p := range f.Passengers {
		// An infant on a lap is not a booking of a seat.
		if p.Status == StatusDeleted || p.Type == PaxInfant {
			continue
		}
		comp := p.Compartment
		if f.Cabin != nil {
			comp = f.Cabin.CompartmentFor(p.Class)
		}
		t := out[comp]
		if p.Category != CategoryListed && p.Category != CategoryInvol || p.DeletedAfterAcceptance {
			if p.Status == StatusBoarded {
				t.Others++
				out[comp] = t
			}
			continue
		}
		t.Booked++
		switch p.Status {
		case StatusNoShow, StatusListed:
			t.NoShows++
		case StatusOffloaded:
			t.Offloaded++
		case StatusDenied:
			t.Denied++
			if p.Volunteer {
				t.Volunteers++
			}
		case StatusBoarded:
			t.Boarded++
		}
		out[comp] = t
	}
	return out
}

// occupied counts the seats taken in a compartment.
func (f *Flight) occupied(comp string) int {
	n := 0
	for seat := range f.Cabin.Occupied {
		if c, ok := f.Cabin.Has(seat); ok && c == comp {
			n++
		}
	}
	return n
}
