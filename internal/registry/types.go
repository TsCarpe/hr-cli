package registry

// Schema 对应 JSON Schema 的子集(YApi 用 draft-04)。
// 强类型映射:已知字段全部建模,未知字段(如 $schema)忽略。
type Schema struct {
	Type        string             `json:"type"`                   // object | string | integer | number | boolean | array
	Description string             `json:"description,omitempty"`  // 字段说明
	Properties  map[string]*Schema `json:"properties,omitempty"`    // type=object 时:字段定义
	Items       *Schema            `json:"items,omitempty"`         // type=array 时:元素 schema
	Required    []string           `json:"required,omitempty"`      // type=object 时:必填字段名列表
	Enum        []string           `json:"enum,omitempty"`          // 枚举值(预解析)
	EnumLabels  []string           `json:"enumLabels,omitempty"`    // 枚举标签(预解析,中文)
}

// Method 一个 API 方法的元数据,1:1 映射 listen REST API 的一个端点。
type Method struct {
	Name            string  `json:"name"`            // 命令名,如 "add"
	Path            string  `json:"path"`            // 相对 basePath 的路径,如 "/add"
	HTTPMethod      string  `json:"httpMethod"`      // GET | POST | PUT | DELETE
	Description     string  `json:"description"`     // 命令描述,--help 时显示
	Risk            string  `json:"risk"`            // read | write
	RequiresAuth    bool    `json:"requiresAuth"`    // 是否需要登录态
	AuthHeader      string  `json:"authHeader,omitempty"` // 认证 header 名,默认空="hrToken";saas 接口标 "Authorization"
	RequestSchema   *Schema `json:"requestSchema"`   // 请求体 JSON Schema
	ResponseSchema  *Schema `json:"responseSchema"`  // 响应体 JSON Schema
}

// Service 一个业务领域的元数据,包含一组 method。
type Service struct {
	Name     string             `json:"name"`     // service 名,如 "course"(注意 URL 路径里的 /course/invite/ 是后端业务路径,与 service 名无关)
	Title    string             `json:"title"`    // 中文标题,如 "课程邀请"
	BasePath string             `json:"basePath"` // URL 前缀,如 "/listen/v1/course/invite"
	Methods  map[string]*Method `json:"methods"`  // method 名 → Method
}

// Registry 顶层容器,所有 service 的元数据集合。
type Registry struct {
	Version  string    `json:"version"`  // 元数据版本(同步时间)
	Services []*Service `json:"services"` // service 列表(数组保留顺序)
}
