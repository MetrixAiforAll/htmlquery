package htmlquery

import (
	"fmt"
	"sync"
	"testing"

	"github.com/antchfx/xpath"
)

// resetCache restores the package-level cache state so tests are isolated.
func resetCache() {
	DisableSelectorCache = false
	SelectorCacheMaxEntries = 50
	cacheOnce = sync.Once{}
	cache = nil
	cacheMutex = sync.Mutex{}
}

func TestGetQuery_CachesExpression(t *testing.T) {
	resetCache()
	SelectorCacheMaxEntries = 10

	expr := "//div[@class='item']"
	e1, err := getQuery(expr)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := getQuery(expr)
	if err != nil {
		t.Fatal(err)
	}
	if e1 != e2 {
		t.Fatal("expected same *xpath.Expr pointer on cache hit")
	}
}

func TestGetQuery_DifferentExpressions(t *testing.T) {
	resetCache()
	SelectorCacheMaxEntries = 10

	e1, err := getQuery("//a")
	if err != nil {
		t.Fatal(err)
	}
	e2, err := getQuery("//div")
	if err != nil {
		t.Fatal(err)
	}
	if e1 == e2 {
		t.Fatal("different expressions should produce different Expr objects")
	}
}

func TestGetQuery_InvalidExpression(t *testing.T) {
	resetCache()

	_, err := getQuery("//[invalid")
	if err == nil {
		t.Fatal("expected error for invalid XPath")
	}

	// Invalid expressions must not be stored in the cache.
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if cache != nil {
		if _, ok := cache.Get("//[invalid"); ok {
			t.Fatal("invalid expression was cached")
		}
	}
}

func TestGetQuery_DisableSelectorCache(t *testing.T) {
	resetCache()
	DisableSelectorCache = true

	expr := "//span"
	e1, err := getQuery(expr)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := getQuery(expr)
	if err != nil {
		t.Fatal(err)
	}
	if e1 == e2 {
		t.Fatal("when cache is disabled, each call should compile a new Expr")
	}
}

func TestGetQuery_ZeroMaxEntriesDisablesCache(t *testing.T) {
	resetCache()
	SelectorCacheMaxEntries = 0

	expr := "//p"
	e1, err := getQuery(expr)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := getQuery(expr)
	if err != nil {
		t.Fatal(err)
	}
	if e1 == e2 {
		t.Fatal("SelectorCacheMaxEntries <= 0 should disable caching")
	}
}

func TestGetQuery_LRUEviction(t *testing.T) {
	resetCache()
	SelectorCacheMaxEntries = 2

	// Fill the cache with two entries.
	_, err := getQuery("//a")
	if err != nil {
		t.Fatal(err)
	}
	_, err = getQuery("//b")
	if err != nil {
		t.Fatal(err)
	}

	// Adding a third should evict the least recently used ("//a").
	_, err = getQuery("//c")
	if err != nil {
		t.Fatal(err)
	}

	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if cache == nil {
		t.Fatal("cache should have been initialized")
	}
	if _, ok := cache.Get("//a"); ok {
		t.Fatal("expected //a to be evicted from LRU cache")
	}
	if _, ok := cache.Get("//b"); !ok {
		t.Fatal("expected //b to still be in cache")
	}
	if _, ok := cache.Get("//c"); !ok {
		t.Fatal("expected //c to be in cache")
	}
}

func TestGetQuery_Concurrent(t *testing.T) {
	resetCache()
	SelectorCacheMaxEntries = 20

	var wg sync.WaitGroup
	const goroutines = 50
	const iterations = 100

	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				expr := fmt.Sprintf("//div[position()=%d]", i%10)
				e, err := getQuery(expr)
				if err != nil {
					t.Errorf("goroutine %d: %v", id, err)
					return
				}
				if e == nil {
					t.Errorf("goroutine %d: nil Expr", id)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestGetQuery_CacheOnceInitialization(t *testing.T) {
	resetCache()
	SelectorCacheMaxEntries = 5

	// First call initializes the cache.
	_, err := getQuery("//html")
	if err != nil {
		t.Fatal(err)
	}
	if cache == nil {
		t.Fatal("cache should be initialized after first getQuery")
	}

	// Changing max entries after init has no effect.
	SelectorCacheMaxEntries = 1
	_, err = getQuery("//body")
	if err != nil {
		t.Fatal(err)
	}
	// Both should still be present because the original capacity was used.
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if _, ok := cache.Get("//html"); !ok {
		t.Fatal("expected original entry to remain")
	}
	if _, ok := cache.Get("//body"); !ok {
		t.Fatal("expected second entry to be cached under original capacity")
	}
}

// Ensure the compiled type is what we expect.
func TestGetQuery_ReturnsXPathExpr(t *testing.T) {
	resetCache()
	e, err := getQuery("//div")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := interface{}(e).(*xpath.Expr); !ok {
		t.Fatal("getQuery should return *xpath.Expr")
	}
}
