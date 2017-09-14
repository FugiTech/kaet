package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"fmt"
)

type store struct {
	sync.RWMutex
	name string
	file *os.File
	data map[string]string
}

func Store(name string) *store {
	f, err := os.OpenFile(name+".json", os.O_RDWR|os.O_CREATE, 0666)
	must(err)
	s := &store{
		name: name,
		file: f,
		data: make(map[string]string),
	}
	json.NewDecoder(f).Decode(&s.data)
	return s
}

func (s *store) save() {
	s.file.Truncate(0)
	s.file.Seek(0, 0)
	json.NewEncoder(s.file).Encode(s.data)
	s.file.Sync()
}

// WRITE
func (s *store) Add(key string, value string) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = value
	s.save()
}
func (s *store) Append(value string) {
	s.Lock()
	defer s.Unlock()
	s.data[strconv.Itoa(len(s.data))] = value
	s.save()
}
func (s *store) Remove(key string) {
	s.Lock()
	defer s.Unlock()
	delete(s.data, key)
	s.save()
}
func (s *store) Blank(key string) bool {
	s.Lock()
	defer s.Unlock()
	val, found := s.data[key]
	doBlank := found && val != ""
	if doBlank {
		s.data[key] = ""
		s.save()
	}
	return doBlank
}

// READ
func (s *store) Keys() []string {
	s.RLock()
	defer s.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k, _ := range s.data {
		keys = append(keys, k)
	}
	sort.Sort(sort.StringSlice(keys))
	return keys
}
func (s *store) Get(key string) (string, bool) {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.data[key]
	return v, ok
}
func (s *store) Random(query string) string {
	query = strings.ToLower(query)
	s.RLock()
	defer s.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k, v := range s.data {
		if v != "" && (query == "" || strings.Contains(strings.ToLower(v), query)) {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "None Found"
	}
	selectedKey := keys[rand.Intn(len(keys))]
	return fmt.Sprintf("%s #%s", s.data[selectedKey], selectedKey)
}
