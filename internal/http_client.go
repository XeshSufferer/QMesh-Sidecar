package internal

import (
	"github.com/valyala/fasthttp"
)

var client *fasthttp.Client = &fasthttp.Client{}

func SendReq(req *fasthttp.Request, resp *fasthttp.Response) error {
	err := client.Do(req, resp)

	return err
}
