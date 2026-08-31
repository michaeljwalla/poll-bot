package set

import (
	"maps"
	"slices"
)

type never = struct{}
type set[T comparable] = map[T]never

type Set[T comparable] struct {
	data *set[T]
}

func New[T comparable](items ...T) Set[T] {
	data := make(set[T], len(items))
	for _, item := range items {
		data[item] = never{}
	}
	return Set[T]{data: &data}
}
func (s Set[T]) Has(item T) bool {
	_, ok := (*s.data)[item]
	return ok
}
func (s Set[T]) Size() int {
	return len(*s.data)
}
func (s Set[T]) Remove(item T) bool {
	had := s.Has(item)
	delete(*s.data, item)
	return had
}
func (s Set[T]) Insert(items ...T) {
	for _, item := range items {
		(*s.data)[item] = never{}
	}
}
func (s Set[T]) TryInsert(items ...T) (success bool) {
	success = true
	for _, item := range items {
		if success = !s.Has(item); !success {
			return
		}
		(*s.data)[item] = never{}
	}
	return
}
func (s Set[T]) Clear() {
	clear(*s.data)
}
func (s Set[T]) Iter() set[T] {
	return *s.data
}
func (s Set[T]) Clone() (clone Set[T]) {
	clone = New[T]()
	clone.UnionLeft(s)
	return
}
func (s Set[T]) UnionLeft(other Set[T]) {
	for item := range *other.data {
		s.Insert(item)
	}
}

func (s Set[T]) IntersectionLeft(other Set[T]) {
	for item := range *other.data {
		if !s.Has(item) {
			s.Remove(item)
		}
	}
}

// compare by items
func (s Set[T]) Equals(other Set[T]) bool {
	if s.PointsTo(other) {
		return true
	}
	if len(*s.data) != len(*other.data) {
		return false
	}
	for item := range *s.data {
		if !other.Has(item) {
			return false
		}
	}
	return true
}

// point to a new underlying map, keeping top level reference
//
// original map loses ref
func (s *Set[T]) PointTo(other Set[T]) {
	s.data = other.data
}
func (s Set[T]) PointsTo(other Set[T]) bool {
	return s.data == other.data
}

func (s Set[T]) IsSubsetOf(other Set[T]) bool {
	if s.PointsTo(other) {
		return true
	}
	if len(*s.data) > len(*other.data) {
		return false
	}
	for item := range *s.data {
		if !other.Has(item) {
			return false
		}
	}
	return true
}

func (s Set[T]) Items() *[]T {
	slice := slices.Collect(maps.Keys(*s.data))
	return &slice
}
