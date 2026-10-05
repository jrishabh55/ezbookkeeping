package api

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/duplicatechecker"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// BucketsApi represents bucket api
type BucketsApi struct {
	ApiUsingConfig
	ApiUsingDuplicateChecker
	buckets  *services.BucketService
	accounts *services.AccountService
}

// Initialize a bucket api singleton instance
var (
	Buckets = &BucketsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		ApiUsingDuplicateChecker: ApiUsingDuplicateChecker{
			ApiUsingConfig: ApiUsingConfig{
				container: settings.Container,
			},
			container: duplicatechecker.Container,
		},
		buckets:  services.Buckets,
		accounts: services.Accounts,
	}
)

// BucketListHandler returns buckets list of current user
func (a *BucketsApi) BucketListHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketListReq models.BucketListRequest
	err := c.ShouldBindQuery(&bucketListReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	buckets, err := a.buckets.GetAllBucketsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[buckets.BucketListHandler] failed to get all buckets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	bucketResps := make(models.BucketInfoResponseSlice, 0, len(buckets))

	for i := 0; i < len(buckets); i++ {
		bucket := buckets[i]

		if bucketListReq.VisibleOnly && !bucket.Visible {
			continue
		}

		bucketResp := bucket.ToBucketInfoResponse()

		// Get linked accounts
		linkedAccounts, err := a.buckets.GetBucketAccountsByBucketId(c, uid, bucket.BucketId)
		if err == nil && len(linkedAccounts) > 0 {
			accountIds := make([]string, len(linkedAccounts))
			for j, la := range linkedAccounts {
				accountIds[j] = utils.Int64ToString(la.AccountId)
			}
			bucketResp.AccountIds = accountIds
		}

		bucketResps = append(bucketResps, bucketResp)
	}

	sort.Sort(bucketResps)

	return bucketResps, nil
}

// BucketGetHandler returns one specific bucket of current user
func (a *BucketsApi) BucketGetHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketGetReq models.BucketGetRequest
	err := c.ShouldBindQuery(&bucketGetReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	bucket, err := a.buckets.GetBucketByBucketId(c, uid, bucketGetReq.Id)

	if err != nil {
		log.Errorf(c, "[buckets.BucketGetHandler] failed to get bucket \"id:%d\" for user \"uid:%d\", because %s", bucketGetReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	bucketResp := bucket.ToBucketInfoResponse()

	// Get linked accounts
	linkedAccounts, err := a.buckets.GetBucketAccountsByBucketId(c, uid, bucket.BucketId)
	if err == nil && len(linkedAccounts) > 0 {
		accountIds := make([]string, len(linkedAccounts))
		for i, la := range linkedAccounts {
			accountIds[i] = utils.Int64ToString(la.AccountId)
		}
		bucketResp.AccountIds = accountIds
	}

	return bucketResp, nil
}

// BucketCreateHandler saves a new bucket by request parameters for current user
func (a *BucketsApi) BucketCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketCreateReq models.BucketCreateRequest
	err := c.ShouldBindJSON(&bucketCreateReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	accountIds, err := parseIdList(bucketCreateReq.AccountIds)

	if err != nil {
		log.Warnf(c, "[buckets.BucketCreateHandler] parse account ids failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	// Check for duplicate submission
	if bucketCreateReq.ClientSessionId != "" {
		found, _ := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUCKET, uid, bucketCreateReq.ClientSessionId)
		if found {
			log.Infof(c, "[buckets.BucketCreateHandler] bucket \"name:%s\" for user \"uid:%d\" has been created, skip duplicate request", bucketCreateReq.Name, uid)
			return nil, errs.ErrDuplicatedSubmission
		}
	}

	maxDisplayOrder, err := a.buckets.GetTotalBucketCountByUid(c, uid)
	if err != nil {
		log.Errorf(c, "[buckets.BucketCreateHandler] failed to get total bucket count for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	bucket := &models.Bucket{
		Uid:          uid,
		Name:         bucketCreateReq.Name,
		TargetAmount: bucketCreateReq.TargetAmount,
		Currency:     bucketCreateReq.Currency,
		Icon:         bucketCreateReq.Icon,
		Color:        bucketCreateReq.Color,
		Comment:      bucketCreateReq.Comment,
		Visible:      true,
		DisplayOrder: int32(maxDisplayOrder) + 1,
	}

	err = a.buckets.CreateBucket(c, bucket, accountIds)

	if err != nil {
		log.Errorf(c, "[buckets.BucketCreateHandler] failed to create bucket \"name:%s\" for user \"uid:%d\", because %s", bucketCreateReq.Name, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketCreateHandler] user \"uid:%d\" has created a new bucket \"id:%d\"", uid, bucket.BucketId)

	if bucketCreateReq.ClientSessionId != "" {
		a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUCKET, uid, bucketCreateReq.ClientSessionId, "")
	}

	bucketResp := bucket.ToBucketInfoResponse()

	return bucketResp, nil
}

// BucketModifyHandler saves an existing bucket by request parameters for current user
func (a *BucketsApi) BucketModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketModifyReq models.BucketModifyRequest
	err := c.ShouldBindJSON(&bucketModifyReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	accountIds, err := parseIdList(bucketModifyReq.AccountIds)

	if err != nil {
		log.Warnf(c, "[buckets.BucketModifyHandler] parse account ids failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	bucket := &models.Bucket{
		BucketId:     bucketModifyReq.Id,
		Uid:          uid,
		Name:         bucketModifyReq.Name,
		TargetAmount: bucketModifyReq.TargetAmount,
		Currency:     bucketModifyReq.Currency,
		Icon:         bucketModifyReq.Icon,
		Color:        bucketModifyReq.Color,
		Comment:      bucketModifyReq.Comment,
		Status:       bucketModifyReq.Status,
		Visible:      bucketModifyReq.Visible,
	}

	err = a.buckets.ModifyBucket(c, bucket, accountIds)

	if err != nil {
		log.Errorf(c, "[buckets.BucketModifyHandler] failed to update bucket \"id:%d\" for user \"uid:%d\", because %s", bucketModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketModifyHandler] user \"uid:%d\" has updated bucket \"id:%d\"", uid, bucketModifyReq.Id)

	bucketResp := bucket.ToBucketInfoResponse()

	return bucketResp, nil
}

// BucketHideHandler hides a bucket by request parameters for current user
func (a *BucketsApi) BucketHideHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketHideReq models.BucketHideRequest
	err := c.ShouldBindJSON(&bucketHideReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketHideHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.buckets.HideBucket(c, uid, bucketHideReq.Id, bucketHideReq.Hidden)

	if err != nil {
		log.Errorf(c, "[buckets.BucketHideHandler] failed to hide bucket \"id:%d\" for user \"uid:%d\", because %s", bucketHideReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketHideHandler] user \"uid:%d\" has hidden bucket \"id:%d\"", uid, bucketHideReq.Id)

	return true, nil
}

// BucketMoveHandler moves display order of buckets by request parameters for current user
func (a *BucketsApi) BucketMoveHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketMoveReq models.BucketMoveRequest
	err := c.ShouldBindJSON(&bucketMoveReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketMoveHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	buckets := make([]*models.Bucket, len(bucketMoveReq.NewDisplayOrders))

	for i := 0; i < len(bucketMoveReq.NewDisplayOrders); i++ {
		buckets[i] = &models.Bucket{
			BucketId:     bucketMoveReq.NewDisplayOrders[i].Id,
			DisplayOrder: bucketMoveReq.NewDisplayOrders[i].DisplayOrder,
		}
	}

	err = a.buckets.ModifyBucketDisplayOrders(c, uid, buckets)

	if err != nil {
		log.Errorf(c, "[buckets.BucketMoveHandler] failed to move buckets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketMoveHandler] user \"uid:%d\" has moved buckets", uid)

	return true, nil
}

// BucketDeleteHandler deletes a bucket by request parameters for current user
func (a *BucketsApi) BucketDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketDeleteReq models.BucketDeleteRequest
	err := c.ShouldBindJSON(&bucketDeleteReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.buckets.DeleteBucket(c, uid, bucketDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[buckets.BucketDeleteHandler] failed to delete bucket \"id:%d\" for user \"uid:%d\", because %s", bucketDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketDeleteHandler] user \"uid:%d\" has deleted bucket \"id:%d\"", uid, bucketDeleteReq.Id)

	return true, nil
}

// BucketAllocateHandler allocates money to a bucket
func (a *BucketsApi) BucketAllocateHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketAllocateReq models.BucketAllocateRequest
	err := c.ShouldBindJSON(&bucketAllocateReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketAllocateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	// Check for duplicate submission
	if bucketAllocateReq.ClientSessionId != "" {
		found, _ := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUCKET_TRANSACTION, uid, bucketAllocateReq.ClientSessionId)
		if found {
			log.Infof(c, "[buckets.BucketAllocateHandler] bucket allocation for user \"uid:%d\" has been created, skip duplicate request", uid)
			return nil, errs.ErrDuplicatedSubmission
		}
	}

	bucketTransaction := &models.BucketTransaction{
		BucketId:          bucketAllocateReq.BucketId,
		Uid:               uid,
		Amount:            bucketAllocateReq.Amount,
		TransactionId:     bucketAllocateReq.TransactionId,
		TransactionTime:   bucketAllocateReq.TransactionTime,
		TimezoneUtcOffset: bucketAllocateReq.UtcOffset,
		Comment:           bucketAllocateReq.Comment,
	}

	err = a.buckets.AllocateToBucket(c, bucketTransaction)

	if err != nil {
		log.Errorf(c, "[buckets.BucketAllocateHandler] failed to allocate to bucket \"id:%d\" for user \"uid:%d\", because %s", bucketAllocateReq.BucketId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketAllocateHandler] user \"uid:%d\" has allocated %d to bucket \"id:%d\"", uid, bucketAllocateReq.Amount, bucketAllocateReq.BucketId)

	if bucketAllocateReq.ClientSessionId != "" {
		a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUCKET_TRANSACTION, uid, bucketAllocateReq.ClientSessionId, "")
	}

	return bucketTransaction.ToBucketTransactionInfoResponse(), nil
}

// BucketWithdrawHandler withdraws money from a bucket
func (a *BucketsApi) BucketWithdrawHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketWithdrawReq models.BucketWithdrawRequest
	err := c.ShouldBindJSON(&bucketWithdrawReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketWithdrawHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	// Check for duplicate submission
	if bucketWithdrawReq.ClientSessionId != "" {
		found, _ := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUCKET_TRANSACTION, uid, bucketWithdrawReq.ClientSessionId)
		if found {
			log.Infof(c, "[buckets.BucketWithdrawHandler] bucket withdrawal for user \"uid:%d\" has been created, skip duplicate request", uid)
			return nil, errs.ErrDuplicatedSubmission
		}
	}

	bucketTransaction := &models.BucketTransaction{
		BucketId:          bucketWithdrawReq.BucketId,
		Uid:               uid,
		Amount:            bucketWithdrawReq.Amount,
		TransactionId:     bucketWithdrawReq.TransactionId,
		TransactionTime:   bucketWithdrawReq.TransactionTime,
		TimezoneUtcOffset: bucketWithdrawReq.UtcOffset,
		Comment:           bucketWithdrawReq.Comment,
	}

	err = a.buckets.WithdrawFromBucket(c, bucketTransaction)

	if err != nil {
		log.Errorf(c, "[buckets.BucketWithdrawHandler] failed to withdraw from bucket \"id:%d\" for user \"uid:%d\", because %s", bucketWithdrawReq.BucketId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketWithdrawHandler] user \"uid:%d\" has withdrawn %d from bucket \"id:%d\"", uid, bucketWithdrawReq.Amount, bucketWithdrawReq.BucketId)

	if bucketWithdrawReq.ClientSessionId != "" {
		a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUCKET_TRANSACTION, uid, bucketWithdrawReq.ClientSessionId, "")
	}

	return bucketTransaction.ToBucketTransactionInfoResponse(), nil
}

// BucketTransactionListHandler returns bucket transactions for a bucket
func (a *BucketsApi) BucketTransactionListHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketTransactionListReq models.BucketTransactionListRequest
	err := c.ShouldBindQuery(&bucketTransactionListReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketTransactionListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	count := bucketTransactionListReq.Count
	if count <= 0 {
		count = 50
	}

	transactions, err := a.buckets.GetBucketTransactionsByBucketId(c, uid, bucketTransactionListReq.BucketId, bucketTransactionListReq.MaxTime, bucketTransactionListReq.MinTime, count)

	if err != nil {
		log.Errorf(c, "[buckets.BucketTransactionListHandler] failed to get bucket transactions for bucket \"id:%d\" for user \"uid:%d\", because %s", bucketTransactionListReq.BucketId, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	transactionResps := make(models.BucketTransactionInfoResponseSlice, len(transactions))

	for i := 0; i < len(transactions); i++ {
		transactionResps[i] = transactions[i].ToBucketTransactionInfoResponse()
	}

	sort.Sort(transactionResps)

	return transactionResps, nil
}

// BucketTransactionDeleteHandler deletes a bucket transaction
func (a *BucketsApi) BucketTransactionDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var bucketTransactionDeleteReq models.BucketTransactionDeleteRequest
	err := c.ShouldBindJSON(&bucketTransactionDeleteReq)

	if err != nil {
		log.Warnf(c, "[buckets.BucketTransactionDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.buckets.DeleteBucketTransaction(c, uid, bucketTransactionDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[buckets.BucketTransactionDeleteHandler] failed to delete bucket transaction \"id:%d\" for user \"uid:%d\", because %s", bucketTransactionDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[buckets.BucketTransactionDeleteHandler] user \"uid:%d\" has deleted bucket transaction \"id:%d\"", uid, bucketTransactionDeleteReq.Id)

	return true, nil
}

// parseIdList converts textual ids to int64 ids, an omitted list (nil) stays nil
func parseIdList(ids []string) ([]int64, error) {
	if ids == nil {
		return nil, nil
	}

	return utils.StringArrayToInt64Array(ids)
}
