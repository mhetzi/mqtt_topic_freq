package main

type UniqueStringArray struct {
	array map[string]bool
}

func (s *UniqueStringArray) AddString(str string) {
	s.array[str] = true
}

func (s *UniqueStringArray) AddStrings(strs ...string) {
	for _, str := range strs {
		s.AddString(str)
	}
}

func (s *UniqueStringArray) HasString(str string) bool {
	b := s.array[str]
	return b
}

func (d *UniqueStringArray) getStrings() ([]string, uint32) {
	return MapKeys(d.array)
}

func (d *UniqueStringArray) length() uint32 {
	_, l := d.getStrings()
	return l
}

func MapKeys[K comparable, V any](m map[K]V) ([]K, uint32) {
	r := make([]K, 0, len(m))
	var idx uint32 = 0
	for k := range m {
		r = append(r, k)
		idx++
	}
	return r, idx
}
