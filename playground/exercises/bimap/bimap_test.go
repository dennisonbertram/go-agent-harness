package main

import (
	"testing"
)

func TestBiMapIntString(t *testing.T) {
	m := NewBiMap[int, string]()

	m.Put(1, "one")
	m.Put(2, "two")
	m.Put(3, "three")

	if m.Len() != 3 {
		t.Fatalf("expected len 3, got %d", m.Len())
	}

	v, ok := m.GetByKey(2)
	if !ok || v != "two" {
		t.Errorf("GetByKey(2) = (%v, %v); want ('two', true)", v, ok)
	}

	k, ok := m.GetByValue("three")
	if !ok || k != 3 {
		t.Errorf("GetByValue('three') = (%v, %v); want (3, true)", k, ok)
	}

	// Put duplicate key with new value replaces old mapping
	m.Put(2, "deux")
	if m.Len() != 3 {
		t.Errorf("expected len 3 after key overwrite, got %d", m.Len())
	}
	v, ok = m.GetByKey(2)
	if !ok || v != "deux" {
		t.Errorf("after overwrite: GetByKey(2) = %v, expected 'deux'", v)
	}
	kn, ok := m.GetByValue("two")
	if ok {
		t.Errorf("expected 'two' to be gone after overwrite; got key %v", kn)
	}

	// Put duplicate value with new key removes old key
	m.Put(4, "deux")
	if m.Len() != 3 {
		t.Errorf("expected len 3 after value overwrite, got %d", m.Len())
	}
	k, ok = m.GetByValue("deux")
	if !ok || k != 4 {
		t.Errorf("after overwrite: GetByValue('deux') = %v, expected 4", k)
	}
	v, ok = m.GetByKey(2)
	if ok {
		t.Errorf("key 2 should be gone after value overwrite")
	}

	m.DeleteByKey(1)
	if _, ok := m.GetByKey(1); ok {
		t.Errorf("DeleteByKey(1) did not remove 1")
	}
	if _, ok := m.GetByValue("one"); ok {
		t.Errorf("DeleteByKey(1) did not remove value 'one'")
	}

	m.DeleteByValue("deux")
	if _, ok := m.GetByKey(4); ok {
		t.Errorf("DeleteByValue('deux') did not remove key 4")
	}
}

func TestBiMapStringString(t *testing.T) {
	bm := NewBiMap[string, string]()
	bm.Put("a", "1")
	bm.Put("b", "2")

	v, ok := bm.GetByKey("a")
	if !ok || v != "1" {
		t.Errorf("GetByKey('a') = %v; want '1'", v)
	}

	k, ok := bm.GetByValue("2")
	if !ok || k != "b" {
		t.Errorf("GetByValue('2') = %v; want 'b'", k)
	}

	// Test overwrite with same value (should just replace old key)
	bm.Put("c", "2")
	if bm.Len() != 2 {
		t.Errorf("Len() = %d; want 2", bm.Len())
	}
	k, ok = bm.GetByValue("2")
	if !ok || k != "c" {
		t.Errorf("GetByValue('2') = %v; want 'c'", k)
	}
	if _, ok := bm.GetByKey("b"); ok {
		t.Errorf("expected key 'b' to be removed after value collision")
	}
}
