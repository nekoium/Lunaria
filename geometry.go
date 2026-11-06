package main

import "math"

type vector struct{ x, y float64 }

func (item *boardImage) world(x, y float64) vector {
	r := item.Angle * math.Pi / 180
	c, s := math.Cos(r), math.Sin(r)
	return vector{item.X + x*c - y*s, item.Y + x*s + y*c}
}
func (item *boardImage) local(p vector) vector {
	r := -item.Angle * math.Pi / 180
	c, s := math.Cos(r), math.Sin(r)
	x, y := p.x-item.X, p.y-item.Y
	return vector{x*c - y*s, x*s + y*c}
}
func (item *boardImage) corners() [4]vector {
	return [4]vector{item.world(-item.W/2, -item.H/2), item.world(item.W/2, -item.H/2), item.world(item.W/2, item.H/2), item.world(-item.W/2, item.H/2)}
}
func (item *boardImage) contains(p vector) bool {
	q := item.local(p)
	return math.Abs(q.x) <= item.W/2 && math.Abs(q.y) <= item.H/2
}
func near(a, b vector) bool { return math.Hypot(a.x-b.x, a.y-b.y) <= 12*uiScale }
func snapAngle(angle float64) float64 {
	angle = math.Mod(angle+360, 360)
	cardinal := math.Round(angle/90) * 90
	if math.Abs(angle-cardinal) <= 6 {
		return math.Mod(cardinal, 360)
	}
	return angle
}
