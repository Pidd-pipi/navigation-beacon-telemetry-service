package domain

import (
	"math"
	"testing"
)

func TestPositionDistance(t *testing.T) {
	// 同一点距离为 0
	a := Position{Lat: 30.5, Lng: 122.1}
	if d := a.DistanceTo(a); d != 0 {
		t.Errorf("同点距离应为 0，got %v", d)
	}
	// 约 0.001 度纬度 ≈ 111 米
	b := Position{Lat: 30.501, Lng: 122.1}
	d := a.DistanceTo(b)
	if d < 100 || d > 125 {
		t.Errorf("0.001 度纬度距离应约 111 米，got %v", d)
	}
}

func TestPositionWithinRadius(t *testing.T) {
	anchor := Position{Lat: 30.5, Lng: 122.1}
	near := Position{Lat: 30.5003, Lng: 122.1} // ~33 米
	far := Position{Lat: 30.501, Lng: 122.1}   // ~111 米
	if !near.IsWithinRadius(anchor, 50) {
		t.Error("33 米应在 50 米半径内")
	}
	if far.IsWithinRadius(anchor, 50) {
		t.Error("111 米不应在 50 米半径内")
	}
}

func TestPositionValid(t *testing.T) {
	if !(Position{Lat: 0, Lng: 0}).Valid() {
		t.Error("0,0 应合法")
	}
	if (Position{Lat: 91, Lng: 0}).Valid() {
		t.Error("lat 91 不应合法")
	}
	if (Position{Lat: 0, Lng: 181}).Valid() {
		t.Error("lng 181 不应合法")
	}
	if d := (Position{Lat: 30.5, Lng: 122.1}).DistanceTo(Position{Lat: 30.4, Lng: 122.2}); math.IsNaN(d) || d <= 0 {
		t.Errorf("正常坐标距离应为有限正数，got %v", d)
	}
}
