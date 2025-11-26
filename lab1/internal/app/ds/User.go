package ds

type User struct {
    UserID   uint   `gorm:"primaryKey;column:user_id"`
    Username string `gorm:"column:username"`
    Email    string `gorm:"column:email"`
    Role     string `gorm:"column:role"`
}

type UserDocument struct {
    UserID      uint   `gorm:"column:user_id"`
    DocumentID  uint   `gorm:"column:document_id"`
    RoleInDoc   string `gorm:"column:role_in_doc"`
}
