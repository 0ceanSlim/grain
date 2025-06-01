package utils

import (
	"fmt"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

func ToStringArray(i interface{}) []string {
	if i == nil {
		return nil
	}
	arr, ok := i.([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, v := range arr {
		str, ok := v.(string)
		if ok {
			result = append(result, str)
		}
	}
	return result
}

func ToIntArray(i interface{}) []int {
	if i == nil {
		return nil
	}
	arr, ok := i.([]interface{})
	if !ok {
		return nil
	}
	var result []int
	for _, v := range arr {
		num, ok := v.(float64)
		if ok {
			result = append(result, int(num))
		}
	}
	return result
}

func ToTagsMap(i interface{}) map[string][]string {
	if i == nil {
		return nil
	}
	tags, ok := i.(map[string]interface{})
	if !ok {
		return nil
	}
	result := make(map[string][]string)
	for k, v := range tags {
		result[k] = ToStringArray(v)
	}
	return result
}

func ToInt64(i interface{}) *int64 {
	if i == nil {
		return nil
	}
	num, ok := i.(float64)
	if !ok {
		return nil
	}
	val := int64(num)
	return &val
}

func ToInt(i interface{}) *int {
	if i == nil {
		return nil
	}
	num, ok := i.(float64)
	if !ok {
		return nil
	}
	val := int(num)
	return &val
}

func ToTime(data interface{}) *time.Time {
	if data == nil {
		return nil
	}
	// Ensure data is a float64 which MongoDB uses for numbers
	timestamp, ok := data.(float64)
	if !ok {
		log.Util().Warn("Invalid timestamp format", "value_type", fmt.Sprintf("%T", data))
		return nil
	}
	t := time.Unix(int64(timestamp), 0).UTC()
	return &t
}