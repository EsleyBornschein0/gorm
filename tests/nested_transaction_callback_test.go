package tests

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

type UserWithCallback struct {
	gorm.Model
	Name string
}

func (u *UserWithCallback) AfterSave(tx *gorm.DB) (err error) {
	if u.Name == "trigger-error" {
		return errors.New("callback error triggered")
	}
	return nil
}

func TestNestedTransactionCallbackRollback(t *testing.T) {
	tx := DB.Begin()
	defer tx.Rollback()

	tx.AutoMigrate(&UserWithCallback{})

	// 1. Start a nested transaction
	err := tx.Transaction(func(nestedTx *gorm.DB) error {
		user := UserWithCallback{Name: "trigger-error"}
		// This save will trigger the AfterSave callback which returns an error
		if err := nestedTx.Save(&user).Error; err != nil {
			return err
		}
		return nil
	})

	// 2. Assertions
	if err == nil {
		t.Fatalf("expected error from nested transaction callback, got nil")
	}

	// 3. Verify database state (ensure rollback occurred)
	var count int64
	DB.Model(&UserWithCallback{}).Where("name = ?", "trigger-error").Count(&count)
	if count != 0 {
		t.Errorf("expected 0 records to be saved due to rollback, found %d", count)
	}
}
