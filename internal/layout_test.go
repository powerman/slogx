package internal

import (
	"slices"
	"testing"
)

// BenchmarkKeyLookup benchmarks different methods of looking up for Prefix and Suffix keys.
func BenchmarkKeyLookup(b *testing.B) {
	var (
		makeMap = func(keys []string) map[string][]byte {
			m := make(map[string][]byte, len(keys))
			for _, k := range keys {
				m[k] = []byte(k)
			}
			return m
		}
		makeSorted = func(keys []string) []string {
			sorted := slices.Clone(keys)
			slices.Sort(sorted)
			return sorted
		}
		lookupSlice  = slices.Contains[[]string]
		lookupBinary = func(slice []string, key string) bool {
			_, ok := slices.BinarySearch(slice, key)
			return ok
		}
		lookupMap = func(m map[string][]byte, key string) bool {
			_, ok := m[key]
			return ok
		}
		keys = []string{
			"server",
			"remoteIP",
			"addr",
			"host",
			"port",
			"handler",
			"op",
			"service",
			"method",
			"grpcCode",
			"httpCode",
			"httpMethod",
			"userID",
			"accountID",
			"requestID",
		}
		keys5      = keys[:5]
		keys10     = keys[:10]
		keys15     = keys[:15]
		sorted5    = makeSorted(keys5)
		sorted10   = makeSorted(keys10)
		sorted15   = makeSorted(keys15)
		map5       = makeMap(keys5)
		map10      = makeMap(keys10)
		map15      = makeMap(keys15)
		lookup5    = keys[5/2]
		lookup10   = keys[10/2]
		lookup15   = keys[15/2]
		lookupMiss = "notfound"
	)
	type caseStruct[T any] struct {
		name   string
		keys   T
		key    string
		lookup func(T, string) bool
		want   bool
	}
	sliceCases := []caseStruct[[]string]{
		{"keys5", keys5, lookup5, lookupSlice, true},
		{"keys5", keys5, lookupMiss, lookupSlice, false},
		{"keys10", keys10, lookup10, lookupSlice, true},
		{"keys10", keys10, lookupMiss, lookupSlice, false},
		{"keys15", keys15, lookup15, lookupSlice, true},
		{"keys15", keys15, lookupMiss, lookupSlice, false},
		{"sorted5", sorted5, lookup5, lookupBinary, true},
		{"sorted5", sorted5, lookupMiss, lookupBinary, false},
		{"sorted10", sorted10, lookup10, lookupBinary, true},
		{"sorted10", sorted10, lookupMiss, lookupBinary, false},
		{"sorted15", sorted15, lookup15, lookupBinary, true},
		{"sorted15", sorted15, lookupMiss, lookupBinary, false},
	}
	mapCases := []caseStruct[map[string][]byte]{
		{"map5", map5, lookup5, lookupMap, true},
		{"map5", map5, lookupMiss, lookupMap, false},
		{"map10", map10, lookup10, lookupMap, true},
		{"map10", map10, lookupMiss, lookupMap, false},
		{"map15", map15, lookup15, lookupMap, true},
		{"map15", map15, lookupMiss, lookupMap, false},
	}
	for _, tc := range sliceCases {
		key := tc.key
		if key != lookupMiss {
			key = "found"
		}
		b.Run(tc.name+"/"+key, func(b *testing.B) {
			for b.Loop() {
				if got := tc.lookup(tc.keys, tc.key); got != tc.want {
					b.Fatalf("got %v; want %v", got, tc.want)
				}
			}
		})
	}
	for _, tc := range mapCases {
		key := tc.key
		if key != lookupMiss {
			key = "found"
		}
		b.Run(tc.name+"/"+key, func(b *testing.B) {
			for b.Loop() {
				if got := tc.lookup(tc.keys, tc.key); got != tc.want {
					b.Fatalf("got %v; want %v", got, tc.want)
				}
			}
		})
	}
}
