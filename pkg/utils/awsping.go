package utils

import (
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

var (
	version   = "1.0.0"
	useragent = fmt.Sprintf("AwsPing/%s", version)
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// Duration2ms converts time.Duration to ms (float64)
func Duration2ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1000 / 1000
}

func mkRandoString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// AWSRegion description of the AWS EC2 region
type AWSRegion struct {
	Name      string
	Code      string
	Service   string
	Latencies []time.Duration
	Error     error
}

// CheckLatencyHTTP Test Latency via HTTP
func (r *AWSRegion) CheckLatencyHTTP(wg *sync.WaitGroup) {
	defer wg.Done()
	url := fmt.Sprintf("http://%s.%s.amazonaws.com/ping?x=%s", r.Service,
		r.Code, mkRandoString(13))
	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", useragent)

	start := time.Now()
	resp, err := client.Do(req)
	r.Latencies = append(r.Latencies, time.Since(start))
	defer resp.Body.Close()

	r.Error = err
}

// CheckLatencyTCP Test Latency via TCP
func (r *AWSRegion) CheckLatencyTCP(wg *sync.WaitGroup) {
	defer wg.Done()
	tcpURI := fmt.Sprintf("%s.%s.amazonaws.com:80", r.Service, r.Code)
	tcpAddr, err := net.ResolveTCPAddr("tcp4", tcpURI)
	if err != nil {
		r.Error = err
		return
	}
	start := time.Now()
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		r.Error = err
		return
	}
	r.Latencies = append(r.Latencies, time.Since(start))
	defer conn.Close()

	r.Error = err
}

// GetLatency returns Latency in ms
func (r *AWSRegion) GetLatency() float64 {
	sum := float64(0)
	for _, l := range r.Latencies {
		sum += Duration2ms(l)
	}
	return sum / float64(len(r.Latencies))
}

// GetLatencyStr returns Latency in string
func (r *AWSRegion) GetLatencyStr() string {
	if r.Error != nil {
		return r.Error.Error()
	}
	return fmt.Sprintf("%.2f ms", r.GetLatency())
}

// AWSRegions slice of the AWSRegion
type AWSRegions []AWSRegion

func (rs AWSRegions) Len() int {
	return len(rs)
}

func (rs AWSRegions) Less(i, j int) bool {
	return rs[i].GetLatency() < rs[j].GetLatency()
}

func (rs AWSRegions) Swap(i, j int) {
	rs[i], rs[j] = rs[j], rs[i]
}

// Get returns region at index
func (rs AWSRegions) Get(i int) AWSRegion {
	return rs[i]
}

// CalcLatency returns list of aws regions sorted by Latency
func CalcLatency(repeats int, useHTTP bool, service string) *AWSRegions {
	regions := AWSRegions{
		{Service: service, Name: "US-East (Virginia)", Code: "us-east-1"},
		{Service: service, Name: "US-East (Ohio)", Code: "us-east-2"},
		{Service: service, Name: "US-West (California)", Code: "us-west-1"},
		{Service: service, Name: "US-West (Oregon)", Code: "us-west-2"},
		{Service: service, Name: "Canada (Ceentral)", Code: "ca-central-1"},
		{Service: service, Name: "Europe (Ireland)", Code: "eu-west-1"},
		{Service: service, Name: "Europe (Frankfurt)", Code: "eu-central-1"},
		{Service: service, Name: "Europe (London)", Code: "eu-west-2"},
		{Service: service, Name: "Asia Pacific (Tokyo)", Code: "ap-northeast-1"},
		{Service: service, Name: "Asia Pacific (Seoul)", Code: "ap-northeast-2"},
		{Service: service, Name: "Asia Pacific (Singapore)", Code: "ap-southeast-1"},
		{Service: service, Name: "Asia Pacific (Mumbai)", Code: "ap-south-1"},
		{Service: service, Name: "Asia Pacific (Sydney)", Code: "ap-southeast-2"},
		// {Service: service, Name: "South America (São Paulo)", Code: "sa-east-1"},
		//{Name: "China (Beijing)", Code: "cn-north-1"},
	}
	var wg sync.WaitGroup

	for n := 1; n <= repeats; n++ {
		wg.Add(len(regions))
		for i := range regions {
			if useHTTP {
				go regions[i].CheckLatencyHTTP(&wg)
			} else {
				go regions[i].CheckLatencyTCP(&wg)
			}
		}
		wg.Wait()
	}

	sort.Sort(regions)
	return &regions
}

// LatencyOutput prints data into console
type LatencyOutput struct {
	Level int
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
