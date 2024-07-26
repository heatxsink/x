package ssh

import (
	"fmt"

	"github.com/gosuri/uilive"
)

type ProgressWriter struct {
	total       int64
	progress    int64
	prefixDoing string
	prefixDone  string
	writer      *uilive.Writer
}

func NewProgressWriter(total int64, prefixDoing, prefixDone string) *ProgressWriter {
	p := &ProgressWriter{
		total:       total,
		writer:      uilive.New(),
		prefixDoing: prefixDoing,
		prefixDone:  prefixDone,
	}
	p.writer.Start()
	return p
}

func (pw *ProgressWriter) Write(data []byte) (int, error) {
	n := len(data)
	pw.progress = pw.progress + int64(n)
	fmt.Fprintf(pw.writer, "%s... %0.2f%%\n", pw.prefixDoing, 100*float64(pw.progress)/float64(pw.total))
	return n, nil
}

func (pw *ProgressWriter) Stop() {
	fmt.Fprintf(pw.writer, "%s.\n", pw.prefixDone)
	pw.writer.Stop()
}
