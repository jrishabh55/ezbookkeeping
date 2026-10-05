package errs

import "net/http"

// Error codes related to buckets
var (
	ErrBucketIdInvalid              = NewNormalError(NormalSubcategoryBucket, 0, http.StatusBadRequest, "bucket id is invalid")
	ErrBucketNotFound               = NewNormalError(NormalSubcategoryBucket, 1, http.StatusBadRequest, "bucket not found")
	ErrBucketAmountInvalid          = NewNormalError(NormalSubcategoryBucket, 2, http.StatusBadRequest, "bucket amount is invalid")
	ErrBucketCurrencyInvalid        = NewNormalError(NormalSubcategoryBucket, 3, http.StatusBadRequest, "bucket currency is invalid")
	ErrBucketStatusInvalid          = NewNormalError(NormalSubcategoryBucket, 4, http.StatusBadRequest, "bucket status is invalid")
	ErrBucketInsufficientBalance    = NewNormalError(NormalSubcategoryBucket, 5, http.StatusBadRequest, "bucket has insufficient balance for withdrawal")
	ErrBucketTransactionNotFound    = NewNormalError(NormalSubcategoryBucket, 6, http.StatusBadRequest, "bucket transaction not found")
	ErrBucketTransactionTypeInvalid = NewNormalError(NormalSubcategoryBucket, 7, http.StatusBadRequest, "bucket transaction type is invalid")
	ErrBucketAccountNotFound        = NewNormalError(NormalSubcategoryBucket, 8, http.StatusBadRequest, "bucket linked account not found")
)
