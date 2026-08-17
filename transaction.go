package gorm

import (
	"database/sql"
	"fmt"
)

// TxCommitter transaction committer
type TxCommitter interface {
	Commit() error
	Rollback() error
}

// Begin begins a transaction
func (db *DB) Begin(opts ...*sql.TxOptions) *DB {
	var (
		tx = db.Session(&Session{NewDB: db.Session.NewDB})
	)

	if tx.Error == nil {
		if preparer, ok := tx.Statement.ConnPool.(TxBeginner); ok && preparer != nil {
			tx.Statement.ConnPool, tx.Error = preparer.BeginTx(tx.Statement.Context, opts...)
		} else if beginner, ok := tx.Statement.ConnPool.(ConnPoolBeginner); ok && beginner != nil {
			tx.Statement.ConnPool, tx.Error = beginner.BeginTx(tx.Statement.Context, opts...)
		} else {
			tx.Error = ErrInvalidTransaction
		}
	}

	return tx
}

// Commit commits a transaction
func (db *DB) Commit() *DB {
	if committer, ok := db.Statement.ConnPool.(TxCommitter); ok && committer != nil {
		db.AddError(committer.Commit())
	} else {
		db.AddError(ErrInvalidTransaction)
	}
	return db
}

// Rollback rollbacks a transaction
func (db *DB) Rollback() *DB {
	if committer, ok := db.Statement.ConnPool.(TxCommitter); ok && committer != nil {
		db.AddError(committer.Rollback())
	} else {
		db.AddError(ErrInvalidTransaction)
	}
	return db
}

// SavePoint save point
func (db *DB) SavePoint(name string) *DB {
	if db.Error == nil {
		if savepointer, ok := db.Statement.ConnPool.(SavePointer); ok && savepointer != nil {
			db.AddError(savepointer.SavePoint(name))
		} else {
			db.AddError(ErrUnsupportedSavePoint)
		}
	}
	return db
}

// RollbackTo rollback to save point
func (db *DB) RollbackTo(name string) *DB {
	if db.Error == nil {
		if savepointer, ok := db.Statement.ConnPool.(SavePointer); ok && savepointer != nil {
			db.AddError(savepointer.RollbackTo(name))
		} else {
			db.AddError(ErrUnsupportedSavePoint)
		}
	}
	return db
}

// Transaction start a transaction as a block, return error will rollback, otherwise to commit.
func (db *DB) Transaction(fc func(tx *DB) error, opts ...*sql.TxOptions) (err error) {
	panicked := true

	if committer, ok := db.Statement.ConnPool.(TxCommitter); ok && committer != nil {
		// nested transaction
		if !db.DisableNestedTransaction {
			spName := fmt.Sprintf("sp%p", fc)
			if err = db.SavePoint(spName); err != nil {
				return err
			}

			defer func() {
				// Imperialize panic or error rollback
				if panicked || err != nil {
					db.RollbackTo(spName)
				}
				if err != nil {
					db.AddError(err)
				}
			}()

			tx := db.Session(&Session{NewDB: db.Session.NewDB})
			err = fc(tx)
			panicked = false
			if err == nil && tx.Error != nil {
				err = tx.Error
			}
			return err
		}
	}

	tx := db.Begin(opts...)
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		// Imperialize panic or error rollback
		if panicked || err != nil {
			tx.Rollback()
		}
	}()

	if err = fc(tx); err == nil && tx.Error == nil {
		panicked = false
		return tx.Commit().Error
	}

	panicked = false
	if err == nil {
		err = tx.Error
	}
	return err
}
