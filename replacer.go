package libio

import (
	"bytes"
	"io"
)

type Replacer interface {
	Replace(reader io.Reader) io.Reader
}

// BytesReplacer allows customization on how StreamReplacingReader does sizing estimate during
// initialization/reset and does search and replacement during the execution.
type BytesReplacer interface {
	// GetSizingHints returns hints for StreamReplacingReader to do sizing estimate and allocation.
	// Return values:
	// - 1st: max search token len
	// - 2nd: max replace token len
	// - 3rd: max (search_len / replace_len) ratio that is less than 1,
	//        if none of the search/replace ratio is less than 1, then return a negative number.
	// will only be called once during StreamReplacingReader initialization/reset.
	GetSizingHints() (int, int, float64)
	// Index does token search for StreamReplacingReader.
	// Return values:
	// - 1st: index of the first found search token; -1, if not found;
	// - 2nd: the found search token; ignored if not found;
	// - 3rd: the matching replace token; ignored if not found;
	Index(buf []byte) (int, []byte, []byte)
}

const defaultBufSize = int(4096)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// StreamReplacingReader allows transparent replacement of a given token during read operation.
type StreamReplacingReader struct {
	replacer          BytesReplacer
	maxSearchTokenLen int
	r                 io.Reader
	err               error
	buf               []byte
	// buf[0:buf0]: bytes already processed; buf[buf0:buf1] bytes read in but not yet processed.
	buf0, buf1 int
	// because we need to replace 'search' with 'replace', this marks the max bytes we can read into buf
	max int
}

func (r *StreamReplacingReader) ResetEx(r1 io.Reader, replacer BytesReplacer) *StreamReplacingReader {
	if r1 == nil {
		panic("io.Reader cannot be nil")
	}
	r.replacer = replacer
	maxSearchTokenLen, maxReplaceTokenLen, maxSearchOverReplaceLenRatio := r.replacer.GetSizingHints()
	if maxSearchTokenLen == 0 {
		panic("search token cannot be nil/empty")
	}
	r.maxSearchTokenLen = maxSearchTokenLen
	r.r = r1
	r.err = nil
	bufSize := max(defaultBufSize, max(maxSearchTokenLen, maxReplaceTokenLen))
	if r.buf == nil || len(r.buf) < bufSize {
		r.buf = make([]byte, bufSize)
	}
	r.buf0 = 0
	r.buf1 = 0
	r.max = len(r.buf)
	if maxSearchOverReplaceLenRatio > 0 {
		// If len(search) < len(replace), then we have to assume the worst case:
		// what's the max bound value such that if we have consecutive 'search' filling up
		// the buf up to buf[:max], and all of them are placed with 'replace', and the final
		// result won't end up exceed the len(buf)?
		r.max = int(maxSearchOverReplaceLenRatio * float64(len(r.buf)))
	}
	return r
}

func (r *StreamReplacingReader) Read(p []byte) (int, error) {
	n := 0
	for {
		if r.buf0 > 0 {
			n = copy(p, r.buf[0:r.buf0])
			r.buf0 -= n
			r.buf1 -= n
			if r.buf1 == 0 && r.err != nil {
				return n, r.err
			}
			copy(r.buf, r.buf[n:r.buf1+n])
			return n, nil
		} else if r.err != nil {
			return 0, r.err
		}

		n, r.err = r.r.Read(r.buf[r.buf1:r.max])
		if n > 0 {
			r.buf1 += n
			for {
				index, search, replace := r.replacer.Index(r.buf[r.buf0:r.buf1])
				if index < 0 {
					r.buf0 = max(r.buf0, r.buf1-r.maxSearchTokenLen+1)
					break
				}
				searchTokenLen := len(search)
				if searchTokenLen == 0 {
					panic("search token cannot be nil/empty")
				}
				replaceTokenLen := len(replace)
				lenDelta := replaceTokenLen - searchTokenLen
				index += r.buf0
				copy(r.buf[index+replaceTokenLen:r.buf1+lenDelta], r.buf[index+searchTokenLen:r.buf1])
				copy(r.buf[index:index+replaceTokenLen], replace)
				r.buf0 = index + replaceTokenLen
				r.buf1 += lenDelta
			}
		}
		if r.err != nil {
			r.buf0 = r.buf1
		}
	}
}

type byteReplace struct {
	search  []byte
	replace []byte
}

func (b *byteReplace) GetSizingHints() (int, int, float64) {
	searchLen := len(b.search)
	replaceLen := len(b.replace)
	ratio := float64(-1)
	if searchLen < replaceLen {
		ratio = float64(searchLen) / float64(replaceLen)
	}
	return searchLen, replaceLen, ratio
}

func (b *byteReplace) Index(buf []byte) (int, []byte, []byte) {
	return bytes.Index(buf, b.search), b.search, b.replace
}

type replacer struct {
	index    int
	replaces []BytesReplacer

	maxSearchLen  int
	maxReplaceLen int
	maxRatio      float64
}

func NewReplacer(oldnews ...string) Replacer {
	res := &replacer{
		replaces: make([]BytesReplacer, 0),
		maxRatio: -1,
	}

	if len(oldnews)%2 == 1 {
		panic("stream.NewReplacer: odd argument count")
	}

	for i := 0; i < len(oldnews); i += 2 {
		if len(oldnews[i]) == 0 { // search can not be empty
			continue
		}
		er := &byteReplace{search: []byte(oldnews[i]), replace: []byte(oldnews[i+1])}
		res.replaces = append(res.replaces, er)
		searchLen, replaceLen, ratio := er.GetSizingHints()
		if searchLen > res.maxSearchLen {
			res.maxSearchLen = searchLen
		}
		if replaceLen > res.maxReplaceLen {
			res.maxReplaceLen = replaceLen
		}
		if ratio > res.maxRatio {
			res.maxRatio = ratio
		}
	}

	return res
}

func (r *replacer) GetSizingHints() (int, int, float64) {
	return r.maxSearchLen, r.maxReplaceLen, r.maxRatio
}

func (r *replacer) Index(buf []byte) (resIndex int, resSearch []byte, resReplace []byte) {
	if len(buf) == 0 {
		return -1, nil, nil
	}
	resIndex = -1
	for _, er := range r.replaces {
		index, search, replace := er.Index(buf)
		if index >= 0 {
			if resIndex == -1 || index < resIndex {
				resIndex, resSearch, resReplace = index, search, replace
			}
		}
	}
	return
}

func (r *replacer) Replace(src io.Reader) io.Reader {
	return (&StreamReplacingReader{}).ResetEx(src, r)
}
