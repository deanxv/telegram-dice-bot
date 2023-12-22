package utils

import (
	"fmt"
	"net/url"
)

// MapToQueryString 将 map 转换为查询字符串
func MapToQueryString(m map[string]string) string {
	values := url.Values{}
	for k, v := range m {
		values.Add(k, fmt.Sprintf("%v", v))
	}
	return values.Encode()
}

// QueryStringToMap 将查询字符串转换为 map
func QueryStringToMap(query string) (map[string]string, error) {
	values, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	m := make(map[string]string)
	for k, v := range values {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m, nil
}
