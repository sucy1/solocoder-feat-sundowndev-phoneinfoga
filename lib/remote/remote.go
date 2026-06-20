package remote

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"github.com/sirupsen/logrus"
	"github.com/sundowndev/phoneinfoga/v2/lib/filter"
	"github.com/sundowndev/phoneinfoga/v2/lib/number"
	"sync"
	"time"
)

var mu sync.Locker = &sync.RWMutex{}
var plugins []Scanner

type DelayConfig struct {
	Fixed    time.Duration
	MinDelay time.Duration
	MaxDelay time.Duration
	IsRandom bool
}

type Library struct {
	m        *sync.RWMutex
	scanners []Scanner
	results  map[string]interface{}
	errors   map[string]error
	filter   filter.Filter
	delay    DelayConfig
}

func NewLibrary(filterEngine filter.Filter) *Library {
	return &Library{
		m:        &sync.RWMutex{},
		scanners: []Scanner{},
		results:  map[string]interface{}{},
		errors:   map[string]error{},
		filter:   filterEngine,
	}
}

func (r *Library) SetDelay(d DelayConfig) {
	r.delay = d
}

func (r *Library) LoadPlugins() {
	for _, s := range plugins {
		r.AddScanner(s)
	}
}

func (r *Library) AddScanner(s Scanner) {
	if r.filter.Match(s.Name()) {
		logrus.WithField("scanner", s.Name()).Debug("Scanner was ignored by filter")
		return
	}
	for i, existing := range r.scanners {
		if existing.Name() == s.Name() {
			logrus.WithField("scanner", s.Name()).Debug("Scanner with same name already exists, overriding")
			r.scanners[i] = s
			return
		}
	}
	r.scanners = append(r.scanners, s)
}

func (r *Library) addResult(k string, v interface{}) {
	r.m.Lock()
	defer r.m.Unlock()
	r.results[k] = v
}

func (r *Library) addError(k string, err error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.errors[k] = err
}

func (r *Library) isRemoteScanner(s Scanner) bool {
	switch s.(type) {
	case *localScanner:
		return false
	case *googlesearchScanner:
		return false
	default:
		return true
	}
}

func (r *Library) applyDelay(s Scanner) {
	if r.delay.Fixed == 0 && r.delay.MinDelay == 0 {
		return
	}
	if !r.isRemoteScanner(s) {
		return
	}

	var d time.Duration
	if r.delay.IsRandom {
		diff := r.delay.MaxDelay - r.delay.MinDelay
		d = r.delay.MinDelay + time.Duration(rand.Int63n(int64(diff)))
	} else {
		d = r.delay.Fixed
	}

	if d > 0 {
		logrus.WithField("scanner", s.Name()).WithField("delay", d).Debug("Applying delay before scanner")
		time.Sleep(d)
	}
}

func (r *Library) Scan(n *number.Number, opts ScannerOptions) (map[string]interface{}, map[string]error) {
	var wg sync.WaitGroup

	for _, s := range r.scanners {
		wg.Add(1)
		go func(s Scanner) {
			defer wg.Done()
			defer func() {
				if err := recover(); err != nil {
					logrus.WithField("scanner", s.Name()).WithField("error", err).Debug("Scanner panicked")
					r.addError(s.Name(), errors.New("panic occurred while running scan, see debug logs"))
				}
			}()

			r.applyDelay(s)

			if err := s.DryRun(*n, opts); err != nil {
				logrus.
					WithField("scanner", s.Name()).
					WithField("reason", err.Error()).
					Debug("Scanner was ignored because it should not run")
				return
			}

			data, err := s.Run(*n, opts)
			if err != nil {
				r.addError(s.Name(), err)
				return
			}
			if data != nil {
				r.addResult(s.Name(), data)
			}
		}(s)
	}

	wg.Wait()

	return r.results, r.errors
}

func (r *Library) GetAllScanners() []Scanner {
	return r.scanners
}

func (r *Library) GetScanner(name string) Scanner {
	r.m.RLock()
	defer r.m.RUnlock()
	for _, s := range r.scanners {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func RegisterPlugin(s Scanner) {
	mu.Lock()
	defer mu.Unlock()
	for i, existing := range plugins {
		if existing.Name() == s.Name() {
			plugins[i] = s
			return
		}
	}
	plugins = append(plugins, s)
}

type DedupSource struct {
	Scanner string      `json:"scanner"`
	Field   string      `json:"field"`
	Value   interface{} `json:"value"`
}

type DedupResult struct {
	Result       map[string]interface{} `json:"results"`
	Errors       map[string]error       `json:"errors,omitempty"`
	BeforeCount  int                    `json:"before_count"`
	AfterCount   int                    `json:"after_count"`
	Sources      map[string][]DedupSource `json:"sources,omitempty"`
}

func DeduplicateResults(result map[string]interface{}, errs map[string]error) DedupResult {
	beforeCount := countFields(result)
	seen := make(map[string]string)
	sources := make(map[string][]DedupSource)
	deduped := make(map[string]interface{})

	for scannerName, data := range result {
		if data == nil {
			continue
		}
		dedupedData, scannerSources := dedupStruct(data, scannerName, "", seen)
		deduped[scannerName] = dedupedData
		if len(scannerSources) > 0 {
			sources[scannerName] = scannerSources
		}
	}

	afterCount := countFields(deduped)

	return DedupResult{
		Result:      deduped,
		Errors:      errs,
		BeforeCount: beforeCount,
		AfterCount:  afterCount,
		Sources:     sources,
	}
}

func dedupStruct(data interface{}, scannerName, prefix string, seen map[string]string) (interface{}, []DedupSource) {
	reflectValue := reflect.ValueOf(data)
	reflectType := reflect.TypeOf(data)

	if reflectValue.Kind() == reflect.Slice {
		var results []interface{}
		var allSources []DedupSource
		for i := 0; i < reflectValue.Len(); i++ {
			item := reflectValue.Index(i)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}
			deduped, srcs := dedupStruct(item.Interface(), scannerName, prefix, seen)
			results = append(results, deduped)
			allSources = append(allSources, srcs...)
		}
		return results, allSources
	}

	newVal := reflect.New(reflectType).Elem()
	var allSources []DedupSource

	for i := 0; i < reflectType.NumField(); i++ {
		field := reflectType.Field(i)
		fieldValue := reflectValue.Field(i)

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		fieldName := jsonTag
		if idx := len(fieldName); idx > 0 {
			if commaIdx := 0; commaIdx >= 0 {
				for j, c := range fieldName {
					if c == ',' {
						commaIdx = j
						break
					}
				}
				if commaIdx > 0 {
					fieldName = fieldName[:commaIdx]
				}
			}
		}
		if fieldName == "" {
			fieldName = field.Name
		}

		fullKey := prefix + "." + fieldName
		if prefix == "" {
			fullKey = fieldName
		}

		switch fieldValue.Kind() {
		case reflect.String:
			val := fieldValue.String()
			if val != "" {
				key := fullKey + ":" + val
				if prev, exists := seen[key]; exists {
					allSources = append(allSources, DedupSource{
						Scanner: scannerName,
						Field:   fullKey,
						Value:   val,
					})
					logrus.WithField("field", fullKey).WithField("value", val).WithField("previous_scanner", prev).WithField("current_scanner", scannerName).Debug("Dedup: field value already seen, skipping")
					continue
				}
				seen[key] = scannerName
				allSources = append(allSources, DedupSource{
					Scanner: scannerName,
					Field:   fullKey,
					Value:   val,
				})
			}
			newVal.Field(i).SetString(val)
		case reflect.Bool:
			val := fieldValue.Bool()
			key := fmt.Sprintf("%s:%v", fullKey, val)
			if _, exists := seen[key]; exists {
				allSources = append(allSources, DedupSource{
					Scanner: scannerName,
					Field:   fullKey,
					Value:   val,
				})
				continue
			}
			seen[key] = scannerName
			newVal.Field(i).SetBool(val)
		case reflect.Int, reflect.Int32, reflect.Int64:
			val := fieldValue.Int()
			key := fmt.Sprintf("%s:%d", fullKey, val)
			if _, exists := seen[key]; exists {
				allSources = append(allSources, DedupSource{
					Scanner: scannerName,
					Field:   fullKey,
					Value:   val,
				})
				continue
			}
			seen[key] = scannerName
			newVal.Field(i).SetInt(val)
		case reflect.Struct:
			deduped, srcs := dedupStruct(fieldValue.Interface(), scannerName, fullKey, seen)
			newVal.Field(i).Set(reflect.ValueOf(deduped))
			allSources = append(allSources, srcs...)
		case reflect.Slice:
			deduped, srcs := dedupStruct(fieldValue.Interface(), scannerName, fullKey, seen)
			if slice, ok := deduped.([]interface{}); ok {
				sliceVal := reflect.MakeSlice(field.Type, 0, len(slice))
				for _, item := range slice {
					if item != nil {
						itemVal := reflect.ValueOf(item)
						if itemVal.Type().ConvertibleTo(field.Type.Elem()) {
							sliceVal = reflect.Append(sliceVal, itemVal.Convert(field.Type.Elem()))
						}
					}
				}
				newVal.Field(i).Set(sliceVal)
			}
			allSources = append(allSources, srcs...)
		default:
			newVal.Field(i).Set(fieldValue)
		}
	}

	return newVal.Interface(), allSources
}

func countFields(result map[string]interface{}) int {
	count := 0
	for _, data := range result {
		if data == nil {
			continue
		}
		count += countStructFields(data)
	}
	return count
}

func countStructFields(data interface{}) int {
	reflectValue := reflect.ValueOf(data)
	reflectType := reflect.TypeOf(data)
	c := 0

	if reflectValue.Kind() == reflect.Slice {
		for i := 0; i < reflectValue.Len(); i++ {
			item := reflectValue.Index(i)
			if item.Kind() == reflect.Ptr {
				item = item.Elem()
			}
			c += countStructFields(item.Interface())
		}
		return c
	}

	for i := 0; i < reflectType.NumField(); i++ {
		jsonTag := reflectType.Field(i).Tag.Get("json")
		if jsonTag == "-" {
			continue
		}
		switch reflectValue.Field(i).Kind() {
		case reflect.Struct:
			c += countStructFields(reflectValue.Field(i).Interface())
		case reflect.Slice:
			c += countStructFields(reflectValue.Field(i).Interface())
		default:
			c++
		}
	}
	return c
}
