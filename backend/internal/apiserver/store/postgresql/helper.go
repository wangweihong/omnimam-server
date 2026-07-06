package postgresql

import "gorm.io/gorm"

// 检查指定字段是否存在对象
func CheckExists(db *gorm.DB, model interface{}, fields map[string]interface{}) bool {
	query := db.Model(model)
	for field, value := range fields {
		query = query.Where(field+" = ?", value)
	}

	var count int64
	query.Count(&count)
	return count > 0
}

// 判断指定名称的对象是否存在
func GetByName(db *gorm.DB, model interface{}, value string) bool {
	query := db.Model(model).Where("name = ?", value)
	var count int64
	query.Count(&count)
	query.First(model)

	return count > 0
}
