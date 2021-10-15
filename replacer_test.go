package libio

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestReplacer(t *testing.T) {
	content := "zt zt ztztzt2zzt zt ztztzt2zt ztzzztzt zt ztztzt2ztzzzzzzzztzzzt zt ztztzzt zt ztztzt2ztztzzzt t2ztztzzztzt zt ztztzt2ztztzzztzttztzzzzt zt ztztzt2ztztzzzttztzt zt ztztzt2ztztzzzt zt ztztzt2ztztzzzt"

	reader := NewReplacer("zt", "zhangtian", "tzzzzzzzztzzzt", "zt2").Replace(strings.NewReader(content))
	res, _ := io.ReadAll(reader)
	res2 := strings.NewReplacer("zt", "zhangtian", "tzzzzzzzztzzzt", "zt2").Replace(content)

	fmt.Println(string(res))
	fmt.Printf(res2)

	if string(res) != res2 {
		t.Errorf("should %s but %s\n", res2, res)
	}

}
