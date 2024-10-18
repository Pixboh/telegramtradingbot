package custom_request

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetCouponInfo(coupon string) *GetCouponResponse {

	url := "https://mhufvbcabpf.com/MobileLiveBet/Mobile_GetCoupon"
	method := "POST"

	payload := strings.NewReader(`{"Guid":"` + coupon + `","Lng":"en","partner":1}`)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return nil
	}
	req.Header.Add("x-language", "en")
	req.Header.Add("x-whence", "22")
	req.Header.Add("x-referral", "1")
	req.Header.Add("x-group", "0")
	req.Header.Add("x-bundleid", "org.xbet.client1")
	req.Header.Add("x-fcountry", "165")
	req.Header.Add("x-devicemanufacturer", "Google")
	req.Header.Add("x-devicemodel", "Sdk_gphone_x86")
	req.Header.Add("x-country", "165")
	// appguid
	appGuid := "1flsf2399f43c8ef_2"
	req.Header.Add("appguid", appGuid)
	now := time.Now()
	currentTimeMillis := int(now.Unix())
	randomIntTenLegnth := rand.Intn(9999999999)
	requestNumber := strconv.Itoa(rand.Intn(100))
	requestUid := "1_" + appGuid + strconv.Itoa(currentTimeMillis) + "_" + requestNumber + "_" + strconv.Itoa(randomIntTenLegnth)
	req.Header.Add(
		"x-request-guid",
		requestUid,
	)
	req.Header.Add("user-agent", "xbet-agent")
	req.Header.Add("version", "1xbet-prod-116(8561)")
	req.Header.Add("content-type", "application/json; charset=UTF-8")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	// get body as json
	var response GetCouponResponse
	err2 := json.Unmarshal(body, &response)
	if err2 != nil && response.Value == nil {
		return nil
	}
	return &response
}
