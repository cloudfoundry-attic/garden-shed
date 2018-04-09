package quotaedreader

import "io"

type QuotaExceededErr struct{}

func (e QuotaExceededErr) Error() string {
	return "layer size exceeds image quota"
}

type QuotaedReader struct {
	DelegateReader io.ReadCloser
	QuotaLeft      int64
}

func NewQuotaExceededErr() QuotaExceededErr {
	return QuotaExceededErr{}
}

func New(delegateReader io.ReadCloser, quotaLeft int64) *QuotaedReader {
	return &QuotaedReader{
		DelegateReader: delegateReader,
		QuotaLeft:      quotaLeft,
	}
}

func (q *QuotaedReader) Read(p []byte) (int, error) {
	if int64(len(p)) > q.QuotaLeft {
		p = p[0 : q.QuotaLeft+1]
	}

	n, err := q.DelegateReader.Read(p)
	q.QuotaLeft = q.QuotaLeft - int64(n)

	if q.QuotaLeft < 0 {
		return n, NewQuotaExceededErr()
	}

	return n, err
}

func (q *QuotaedReader) Close() error {
	return q.DelegateReader.Close()
}
