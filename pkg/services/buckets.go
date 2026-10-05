package services

import (
	"fmt"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// BucketService represents bucket service
type BucketService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a bucket service singleton instance
var (
	Buckets = &BucketService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetTotalBucketCountByUid returns total bucket count of user
func (s *BucketService) GetTotalBucketCountByUid(c core.Context, uid int64) (int64, error) {
	if uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	count, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).Count(&models.Bucket{})

	return count, err
}

// GetAllBucketsByUid returns all bucket models of user
func (s *BucketService) GetAllBucketsByUid(c core.Context, uid int64) ([]*models.Bucket, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var buckets []*models.Bucket
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("display_order asc").Find(&buckets)

	return buckets, err
}

// GetBucketByBucketId returns bucket model according to bucket id
func (s *BucketService) GetBucketByBucketId(c core.Context, uid int64, bucketId int64) (*models.Bucket, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if bucketId <= 0 {
		return nil, errs.ErrBucketIdInvalid
	}

	bucket := &models.Bucket{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND bucket_id=?", uid, false, bucketId).Get(bucket)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrBucketNotFound
	}

	return bucket, nil
}

// GetBucketAccountsByBucketId returns all linked accounts for a bucket
func (s *BucketService) GetBucketAccountsByBucketId(c core.Context, uid int64, bucketId int64) ([]*models.BucketAccount, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if bucketId <= 0 {
		return nil, errs.ErrBucketIdInvalid
	}

	var accounts []*models.BucketAccount
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND bucket_id=?", uid, bucketId).Find(&accounts)

	return accounts, err
}

// GetBucketTransactionsByBucketId returns bucket transactions for a bucket
func (s *BucketService) GetBucketTransactionsByBucketId(c core.Context, uid int64, bucketId int64, maxTime int64, minTime int64, count int32) ([]*models.BucketTransaction, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if bucketId <= 0 {
		return nil, errs.ErrBucketIdInvalid
	}

	var transactions []*models.BucketTransaction
	sess := s.UserDataDB(uid).NewSession(c).Where("uid=? AND bucket_id=?", uid, bucketId)

	if maxTime > 0 {
		sess = sess.And("transaction_time<=?", maxTime)
	}

	if minTime > 0 {
		sess = sess.And("transaction_time>=?", minTime)
	}

	if count > 0 {
		sess = sess.Limit(int(count))
	}

	err := sess.OrderBy("transaction_time desc").Find(&transactions)

	return transactions, err
}

// CreateBucket saves a new bucket to database
func (s *BucketService) CreateBucket(c core.Context, bucket *models.Bucket, accountIds []int64) error {
	if bucket.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	bucket.BucketId = s.GenerateUuid(uuid.UUID_TYPE_BUCKET)

	if bucket.BucketId < 1 {
		return errs.ErrSystemIsBusy
	}

	bucket.Deleted = false
	bucket.CurrentBalance = 0
	bucket.Status = models.BUCKET_STATUS_IN_PROGRESS
	bucket.CreatedUnixTime = time.Now().Unix()
	bucket.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(bucket.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(bucket)

		if err != nil {
			return err
		}

		if len(accountIds) > 0 {
			bucketAccounts := make([]*models.BucketAccount, len(accountIds))
			for i := 0; i < len(accountIds); i++ {
				bucketAccounts[i] = &models.BucketAccount{
					Id:              s.GenerateUuid(uuid.UUID_TYPE_BUCKET),
					BucketId:        bucket.BucketId,
					Uid:             bucket.Uid,
					AccountId:       accountIds[i],
					CreatedUnixTime: time.Now().Unix(),
				}
			}

			_, err = sess.Insert(bucketAccounts)

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// ModifyBucket saves an existing bucket to database
func (s *BucketService) ModifyBucket(c core.Context, bucket *models.Bucket, accountIds []int64) error {
	if bucket.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if bucket.BucketId <= 0 {
		return errs.ErrBucketIdInvalid
	}

	bucket.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(bucket.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		existingBucket := &models.Bucket{}
		has, err := sess.Where("uid=? AND deleted=? AND bucket_id=?", bucket.Uid, false, bucket.BucketId).Get(existingBucket)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrBucketNotFound
		}

		// Balance is only changed by allocations / withdrawals, and status follows it when there is a target
		bucket.CurrentBalance = existingBucket.CurrentBalance

		if bucket.TargetAmount > 0 {
			if bucket.CurrentBalance >= bucket.TargetAmount {
				bucket.Status = models.BUCKET_STATUS_COMPLETED
			} else {
				bucket.Status = models.BUCKET_STATUS_IN_PROGRESS
			}
		}

		updatedRows, err := sess.ID(bucket.BucketId).Where("uid=? AND deleted=?", bucket.Uid, false).Cols("name", "target_amount", "currency", "icon", "color", "comment", "status", "visible", "updated_unix_time").Update(bucket)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrBucketNotFound
		}

		// nil means the client did not send linked accounts, keep the existing ones
		if accountIds == nil {
			return nil
		}

		// Delete existing linked accounts and recreate
		_, err = sess.Where("uid=? AND bucket_id=?", bucket.Uid, bucket.BucketId).Delete(&models.BucketAccount{})

		if err != nil {
			return err
		}

		if len(accountIds) > 0 {
			bucketAccounts := make([]*models.BucketAccount, len(accountIds))
			for i := 0; i < len(accountIds); i++ {
				bucketAccounts[i] = &models.BucketAccount{
					Id:              s.GenerateUuid(uuid.UUID_TYPE_BUCKET),
					BucketId:        bucket.BucketId,
					Uid:             bucket.Uid,
					AccountId:       accountIds[i],
					CreatedUnixTime: time.Now().Unix(),
				}
			}

			_, err = sess.Insert(bucketAccounts)

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// HideBucket updates visibility of a bucket
func (s *BucketService) HideBucket(c core.Context, uid int64, bucketId int64, hidden bool) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if bucketId <= 0 {
		return errs.ErrBucketIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.Bucket{
		Visible:         !hidden,
		UpdatedUnixTime: now,
	}

	updatedRows, err := s.UserDataDB(uid).NewSession(c).ID(bucketId).Where("uid=? AND deleted=?", uid, false).Cols("visible", "updated_unix_time").Update(updateModel)

	if err != nil {
		return err
	} else if updatedRows < 1 {
		return errs.ErrBucketNotFound
	}

	return nil
}

// ModifyBucketDisplayOrders updates display orders of buckets
func (s *BucketService) ModifyBucketDisplayOrders(c core.Context, uid int64, buckets []*models.Bucket) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	for i := 0; i < len(buckets); i++ {
		buckets[i].UpdatedUnixTime = time.Now().Unix()
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		for i := 0; i < len(buckets); i++ {
			bucket := buckets[i]
			updatedRows, err := sess.ID(bucket.BucketId).Where("uid=? AND deleted=?", uid, false).Cols("display_order", "updated_unix_time").Update(bucket)

			if err != nil {
				return err
			} else if updatedRows < 1 {
				return errs.ErrBucketNotFound
			}
		}

		return nil
	})
}

// DeleteBucket deletes a bucket
func (s *BucketService) DeleteBucket(c core.Context, uid int64, bucketId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if bucketId <= 0 {
		return errs.ErrBucketIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.Bucket{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(bucketId).Where("uid=? AND deleted=?", uid, false).Cols("deleted", "deleted_unix_time").Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrBucketNotFound
		}

		// Delete linked accounts
		_, err = sess.Where("uid=? AND bucket_id=?", uid, bucketId).Delete(&models.BucketAccount{})

		return err
	})
}

// AllocateToBucket adds money to a bucket
func (s *BucketService) AllocateToBucket(c core.Context, bucketTransaction *models.BucketTransaction) error {
	if bucketTransaction.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if bucketTransaction.BucketId <= 0 {
		return errs.ErrBucketIdInvalid
	}

	if bucketTransaction.Amount <= 0 {
		return errs.ErrBucketAmountInvalid
	}

	bucketTransaction.BucketTransactionId = s.GenerateUuid(uuid.UUID_TYPE_BUCKET)

	if bucketTransaction.BucketTransactionId < 1 {
		return errs.ErrSystemIsBusy
	}

	bucketTransaction.Type = models.BUCKET_TRANSACTION_TYPE_ALLOCATE
	bucketTransaction.CreatedUnixTime = time.Now().Unix()
	bucketTransaction.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(bucketTransaction.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := s.applyBucketBalanceDelta(sess, bucketTransaction.Uid, bucketTransaction.BucketId, bucketTransaction.Amount); err != nil {
			return err
		}

		// Insert transaction record
		_, err := sess.Insert(bucketTransaction)

		return err
	})
}

// WithdrawFromBucket removes money from a bucket
func (s *BucketService) WithdrawFromBucket(c core.Context, bucketTransaction *models.BucketTransaction) error {
	if bucketTransaction.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if bucketTransaction.BucketId <= 0 {
		return errs.ErrBucketIdInvalid
	}

	if bucketTransaction.Amount <= 0 {
		return errs.ErrBucketAmountInvalid
	}

	bucketTransaction.BucketTransactionId = s.GenerateUuid(uuid.UUID_TYPE_BUCKET)

	if bucketTransaction.BucketTransactionId < 1 {
		return errs.ErrSystemIsBusy
	}

	bucketTransaction.Type = models.BUCKET_TRANSACTION_TYPE_WITHDRAW
	bucketTransaction.CreatedUnixTime = time.Now().Unix()
	bucketTransaction.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(bucketTransaction.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := s.applyBucketBalanceDelta(sess, bucketTransaction.Uid, bucketTransaction.BucketId, -bucketTransaction.Amount); err != nil {
			return err
		}

		// Insert transaction record
		_, err := sess.Insert(bucketTransaction)

		return err
	})
}

// DeleteBucketTransaction deletes a bucket transaction and reverses the balance change
func (s *BucketService) DeleteBucketTransaction(c core.Context, uid int64, transactionId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if transactionId <= 0 {
		return errs.ErrBucketTransactionNotFound
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		// Get the transaction
		bucketTransaction := &models.BucketTransaction{}
		has, err := sess.Where("uid=? AND bucket_transaction_id=?", uid, transactionId).Get(bucketTransaction)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrBucketTransactionNotFound
		}

		// Reverse the balance change
		delta := -bucketTransaction.Amount

		if bucketTransaction.Type == models.BUCKET_TRANSACTION_TYPE_WITHDRAW {
			delta = bucketTransaction.Amount
		}

		if err := s.applyBucketBalanceDelta(sess, uid, bucketTransaction.BucketId, delta); err != nil {
			return err
		}

		// Delete the transaction
		_, err = sess.Where("uid=? AND bucket_transaction_id=?", uid, transactionId).Delete(&models.BucketTransaction{})

		return err
	})
}

// applyBucketBalanceDelta changes the bucket balance atomically, refusing to go below zero, then updates the goal status
func (s *BucketService) applyBucketBalanceDelta(sess *xorm.Session, uid int64, bucketId int64, delta int64) error {
	now := time.Now().Unix()

	updatedRows, err := sess.Table(&models.Bucket{}).
		Where("uid=? AND deleted=? AND bucket_id=? AND current_balance+(?)>=0", uid, false, bucketId, delta).
		SetExpr("current_balance", fmt.Sprintf("current_balance+(%d)", delta)).
		Update(map[string]any{"updated_unix_time": now})

	if err != nil {
		return err
	}

	bucket := &models.Bucket{}
	has, err := sess.Where("uid=? AND deleted=? AND bucket_id=?", uid, false, bucketId).Get(bucket)

	if err != nil {
		return err
	} else if !has {
		return errs.ErrBucketNotFound
	} else if updatedRows < 1 {
		return errs.ErrBucketInsufficientBalance
	}

	if bucket.TargetAmount <= 0 {
		return nil
	}

	status := models.BUCKET_STATUS_IN_PROGRESS

	if bucket.CurrentBalance >= bucket.TargetAmount {
		status = models.BUCKET_STATUS_COMPLETED
	}

	_, err = sess.Table(&models.Bucket{}).Where("uid=? AND bucket_id=?", uid, bucketId).Update(map[string]any{"status": status})

	return err
}
