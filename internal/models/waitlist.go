package models

import "gorm.io/gorm"

type WaitlistEntry struct {
	gorm.Model
	Email     string `gorm:"not null;unique;index"`
	FirstName string `gorm:"not null"`
	LastName  string `gorm:"not null"`
}
