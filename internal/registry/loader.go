package registry

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed meta_data.json
var metaDataBytes []byte

var onceLoad = sync.OnceValues(func() (*Registry, error) {
	var r Registry
	jErr := json.Unmarshal(metaDataBytes, &r)
	if jErr != nil {
		return nil, fmt.Errorf("解析元数据失败:%w", jErr)
	}

	return &r, nil
})

func Load() (r *Registry, err error) {

	return onceLoad()
}
