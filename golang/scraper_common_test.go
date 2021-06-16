package csg

//unit tests

import (
	"gopkg.in/yaml.v2"
	"net/url"
	"regexp"
	"testing"
	"time"
)

func TestGetMapRequired(t *testing.T) {
	mapObj := make(map[interface{}]interface{})
	notMapObj := "bar"
	parent := make(map[string]interface{})

	parent["foo"] = mapObj
	parent["foo2"] = notMapObj

	value, err := getMapRequired(parent, "bar")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getMapRequired(parent, "foo2")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getMapRequired(parent, "foo")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
		return
	}
	if value == nil {
		t.Errorf("Expected non-nil map, got nil")
		return
	}
}

func TestGetMapOptional(t *testing.T) {
	mapObj := make(map[interface{}]interface{})
	notMapObj := "bar"
	parent := make(map[string]interface{})

	parent["foo"] = mapObj
	parent["foo2"] = notMapObj

	value := getMapOptional(parent, "bar")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getMapOptional(parent, "foo2")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getMapOptional(parent, "foo")
	if value == nil {
		t.Errorf("Expected non-nil map, got nil")
		return
	}
}

func TestGetMapArrayRequired(t *testing.T) {
	arrayOfNotMaps := []interface{}{"0", "1", "2"}
	foo := map[interface{}]interface{}{0: 0, 1: 1, 2: 2}

	arrayOfWrongKeyTypeMaps := []interface{}{foo, foo, foo}
	bar := map[interface{}]interface{}{"0": 0, "1": 1, "2": 2}

	arrayOfCorrectMaps := []interface{}{bar, bar, bar}

	parent := make(map[string]interface{})
	parent["foo2"] = arrayOfNotMaps
	parent["foo"] = arrayOfWrongKeyTypeMaps
	parent["bar"] = arrayOfCorrectMaps

	value, err := getMapArrayRequired(parent, "baz")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getMapArrayRequired(parent, "foo2")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getMapArrayRequired(parent, "foo")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getMapArrayRequired(parent, "bar")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
		return
	}
	if value == nil || len(value) < 1 {
		t.Errorf("Expected non-nil non-empty array, got %v", value)
		return
	}
}

func TestGetMapArrayOptional(t *testing.T) {
	arrayOfNotMaps := []interface{}{"0", "1", "2"}
	foo := map[interface{}]interface{}{0: 0, 1: 1, 2: 2}

	arrayOfWrongKeyTypeMaps := []interface{}{foo, foo, foo}
	bar := map[interface{}]interface{}{"0": 0, "1": 1, "2": 2}

	arrayOfCorrectMaps := []interface{}{bar, bar, bar}

	parent := make(map[string]interface{})
	parent["foo2"] = arrayOfNotMaps
	parent["foo"] = arrayOfWrongKeyTypeMaps
	parent["bar"] = arrayOfCorrectMaps

	value := getMapArrayOptional(parent, "baz")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getMapArrayOptional(parent, "foo2")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getMapArrayOptional(parent, "foo")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getMapArrayOptional(parent, "bar")
	if value == nil || len(value) < 1 {
		t.Errorf("Expected non-nil non-empty array, got %v", value)
		return
	}
}

func TestGetPatternRequired(t *testing.T) {
	parent := make(map[string]interface{})
	parent["foo"] = "foo"
	parent["foo2"] = "[[[[["
	parent["bar"] = 1

	value, err := getPatternRequired(parent, "baz")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getPatternRequired(parent, "bar")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getPatternRequired(parent, "foo2")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value, err = getPatternRequired(parent, "foo")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
		return
	}
	if value == nil {
		t.Errorf("Expected non-nil pattern, got %v", value)
		return
	}
}

func TestGetPatternOptional(t *testing.T) {
	parent := make(map[string]interface{})
	parent["foo"] = "foo"
	parent["foo2"] = "[[[[["
	parent["bar"] = 1

	value := getPatternOptional(parent, "baz")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getPatternOptional(parent, "bar")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getPatternOptional(parent, "foo2")
	if value != nil {
		t.Errorf("Expected nil value, got %v", value)
		return
	}

	value = getPatternOptional(parent, "foo")
	if value == nil {
		t.Errorf("Expected non-nil pattern, got %v", value)
		return
	}
}

func TestGetIntOptionalWithDefault(t *testing.T) {
	parent := make(map[string]interface{})
	parent["foo"] = 1
	parent["foo2"] = -1
	parent["bar"] = "2"
	defaultValue := -1

	value, exists := getIntOptionalWithDefault(parent, "baz", defaultValue)
	if exists {
		t.Errorf("Expected exists to be false, got true")
		return
	}
	if value != defaultValue {
		t.Errorf("Expected default value (%d), got %v", defaultValue, value)
		return
	}

	value, exists = getIntOptionalWithDefault(parent, "bar", defaultValue)
	if exists {
		t.Errorf("Expected exists to be false, got true")
		return
	}
	if value != defaultValue {
		t.Errorf("Expected default value (%d), got %v", defaultValue, value)
		return
	}

	value, exists = getIntOptionalWithDefault(parent, "foo2", defaultValue)
	if !exists {
		t.Errorf("Expected exists to be true, got false")
		return
	}
	if value != parent["foo2"] {
		t.Errorf("Expected %d, got %v", parent["foo2"], value)
		return
	}

	value, exists = getIntOptionalWithDefault(parent, "foo", defaultValue)
	if !exists {
		t.Errorf("Expected exists to be true, got false")
		return
	}
	if value != parent["foo"] {
		t.Errorf("Expected %d, got %v", parent["foo"], value)
		return
	}
}

func TestGetIntArrayRequired(t *testing.T) {
	parent := make(map[string]interface{})
	parent["foo"] = 1
	parent["foo2"] = []interface{}{1, 2}
	parent["bar"] = []interface{}{"1", 2}
	parent["bar2"] = []interface{}{"1", "2"}

	_, err := getIntArrayRequired(parent, "foo")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	_, err = getIntArrayRequired(parent, "bar")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	_, err = getIntArrayRequired(parent, "bar2")
	if err == nil {
		t.Errorf("Expected error, got nil")
		return
	}
	value, err := getIntArrayRequired(parent, "foo2")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
		return
	}
	if value == nil {
		t.Errorf("Expected non-nil array, got %v", value)
		return
	}
}

const TestEndpointYAML = `endpoint:
  url: "foo"
  method: "POST"
  body: "bar"
  headers:
    Content-Type: "application/lol"
    Cookie: "cookie=yummy"`

func TestGetEndpointRequired(t *testing.T) {
	//happy path testing
	params := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(TestEndpointYAML), params)
	if err != nil {
		panic(err)
	}

	endpoint, err := getEndpointRequired(params, "endpoint")
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
		return
	}

	if endpoint.Url != "foo" {
		t.Errorf("Expected endpoint.Url to be 'foo', got '%v'", endpoint.Url)
		return
	}

	if endpoint.Method != "POST" {
		t.Errorf("Expected endpoint.Method to be 'POST', got '%v'", endpoint.Method)
		return
	}

	if endpoint.Body != "bar" {
		t.Errorf("Expected endpoint.Body to be 'bar', got '%v'", endpoint.Body)
		return
	}

	if len(endpoint.Headers) != 2 {
		t.Errorf("Expected endpoint.Headers to have length 2, got %d", len(endpoint.Headers))
		return
	}

	headers := make(map[string]string)

	for _, header := range endpoint.Headers {
		headers[header.Name] = header.Value
	}

	if headers["Cookie"] != "cookie=yummy" {
		t.Errorf("Expected 'Cookie' header to have value 'cookie=yummy', got %s", headers["Cookie"])
		return
	}

	if headers["Cookie"] != "cookie=yummy" {
		t.Errorf("Expected 'Content-Type' header to have value 'application/lol', got %s", headers["Content-Type"])
		return
	}
}

func TestGetEndpointOptional(t *testing.T) {
	//happy path testing
	params := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(TestEndpointYAML), params)
	if err != nil {
		panic(err)
	}

	endpoint := getEndpointOptional(params, "endpoint")
	if endpoint == nil {
		t.Errorf("Expected non-nil endpoint, got nil")
		return
	}

	if endpoint.Url != "foo" {
		t.Errorf("Expected endpoint.Url to be 'foo', got '%v'", endpoint.Url)
		return
	}

	if endpoint.Method != "POST" {
		t.Errorf("Expected endpoint.Method to be 'POST', got '%v'", endpoint.Method)
		return
	}

	if endpoint.Body != "bar" {
		t.Errorf("Expected endpoint.Body to be 'bar', got '%v'", endpoint.Body)
		return
	}

	if len(endpoint.Headers) != 2 {
		t.Errorf("Expected endpoint.Headers to have length 2, got %d", len(endpoint.Headers))
		return
	}

	headers := make(map[string]string)

	for _, header := range endpoint.Headers {
		headers[header.Name] = header.Value
	}

	if headers["Cookie"] != "cookie=yummy" {
		t.Errorf("Expected 'Cookie' header to have value 'cookie=yummy', got %s", headers["Cookie"])
		return
	}

	if headers["Cookie"] != "cookie=yummy" {
		t.Errorf("Expected 'Content-Type' header to have value 'application/lol', got %s", headers["Content-Type"])
		return
	}
}

func TestGetClinicsByKeyPattern(t *testing.T) {
	var err error

	config, err = NewConfigDefaultPath()
	if err != nil {
		Log.Errorf("Can't read config: %v", err)
		panic(err)
	}

	re := regexp.MustCompile(`walgreens_[0-9]+`)

	clinics, err := GetClinicsByKeyPattern(re)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	if len(clinics) < 1 {
		t.Errorf("Expecting at least 1 location matching '%v' , got %d", re, len(clinics))
		return
	}
}

func TestReplaceMagic(t *testing.T) {
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	processed := replaceMagicWithTime("##CURRENT_DATE##", now)
	expected := now.Format("2006-01-02")

	if processed != expected {
		t.Errorf("Expecting %s, got %s", expected, processed)
		return
	}

	processed = replaceMagicWithTime("##{2006-01-02;1;2}##", now)
	expected = now.AddDate(0, 1, 2).Format("2006-01-02")
	if processed != expected {
		t.Errorf("Expecting %s, got %s", expected, processed)
		return
	}

	processed = replaceMagicWithTime("##{02-Jan-2006 15:04:05;1;2}##", now)
	expected = url.QueryEscape(now.AddDate(0, 1, 2).Format("02-Jan-2006 15:04:05"))
	if processed != expected {
		t.Errorf("Expecting %s, got %s", expected, processed)
		return
	}
}

func TestGetRegexSubmatches(t *testing.T) {
	pattern := regexp.MustCompile(`[a-z]+(?P<named1>[0-9]+)[a-z]+([0-9]+)?[a-z]+([0-9]+)?`)

	testString := "abc123jax456sdf789"

	matches := GetRegexSubmatches(pattern, testString)
	Log.Debugf("%v", matches)
	if len(matches) != 3 {
		t.Errorf("Expecting %d matches, got %d: %v", 3, len(matches), matches)
		return
	}

	if v, ok := matches["named1"]; !ok {
		t.Errorf("Expecting map to contain key 'named1', got %v", matches)
		return
	} else if v != "123" {
		t.Errorf("Expecting map value for 'named1' to be '123', got %v", matches)
		return
	}

	testString = "abc123jax456sdf789 dsa12dsf34"
	matchesArr := GetAllRegexSubmatches(pattern, testString)
	Log.Debugf("%v", matchesArr)
	if len(matchesArr) != 2 {
		t.Errorf("Expecting %d matches, got %d: %v", 2, len(matchesArr), matches)
		return
	}

	pattern = regexp.MustCompile(`foo`)
	testString = "foo"
	matches = GetRegexSubmatches(pattern, testString)
	if len(matches) != 0 {
		t.Errorf("Expecting %d matches, got %d: %v", 0, len(matches), matches)
		return
	}

	pattern = regexp.MustCompile(`(foo)`)
	testString = "foofoofoofoo"
	matchesArr = GetAllRegexSubmatches(pattern, testString)
	Log.Debugf("%v", matchesArr)
}

func TestTagSetMerge(t *testing.T) {
	var bar TagSet
	bar = bar.Add(TagPfizer)

	if len(bar.ToStringArray()) != 1 {
		t.Errorf("Expecting set of size 1, got %d: %v", len(bar.ToStringArray()), bar)
		return
	}

	var foo TagSet
	foo = foo.ParseAndAddVaccineType("blah blah j&j blah")
	if len(foo.ToStringArray()) != 1 {
		t.Errorf("Expecting set of size 1, got %d: %v", len(foo.ToStringArray()), foo)
		return
	}

	if !foo.Contains(TagJohnson) {
		t.Errorf("Expecting set to contain %s, got %v", TagJohnson, foo)
	}

	baz := foo.Merge(bar)

	if len(foo.ToStringArray()) != 1 {
		t.Errorf("Expecting set of size 1, got %d: %v", len(foo.ToStringArray()), foo)
		return
	}

	if len(baz.ToStringArray()) != 2 {
		t.Errorf("Expecting set of size 2, got %d: %v", len(baz.ToStringArray()), baz)
		return
	}

	if !baz.Contains(TagJohnson) {
		t.Errorf("Expecting set to contain %s, got %v", TagJohnson, baz)
	}

	if !baz.Contains(TagPfizer) {
		t.Errorf("Expecting set to contain %s, got %v", TagPfizer, baz)
	}

	Log.Debugf("%v %v %v", foo, bar, baz)
}
