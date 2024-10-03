package dns_utils

import (
	_ "embed"
	"fmt"
	"github.com/analog-substance/radon/pkg/common"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/miekg/unbound"
)

type (
	Result struct {
		domain       string
		originalLine string
		addresses    []string
		txt          []string
		mx           []string
		cName        string
		hosts        []string
		addrErr      error
		txtErr       error
		mxErr        error
		cNameErr     error
		hostsErr     error
	}
)

var (
	DomainCleanerRe = regexp.MustCompile(`^([0-9]+,)?([^\/]*)(?:\/.*)?$`)
	IPMatch         = regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}$`)
	unboundInstance = unbound.New()
	outputLock      sync.Mutex
	ch              chan Result
)

func lookupHost(domain string, attemptNumber int, lookupTimeout time.Duration, maxAttempts int) (addrs []string, err error) {
	timeout := make(chan error)
	go func() {
		addrs, err = unboundInstance.LookupHost(domain)
		timeout <- err
	}()

	select {
	case err = <-timeout:
		if err != nil {
			if attemptNumber < maxAttempts {
				return lookupHost(domain, attemptNumber+1, lookupTimeout, maxAttempts)
			}
			syncPrintf("!!!!! failed: max attempts exhausted for domain=%s error=%s\n", domain, err)
		}
	case <-time.After(lookupTimeout * time.Second):
		err = fmt.Errorf("!!!!! error: timed out for \"%v\" after %v seconds", domain, lookupTimeout)
	}
	return addrs, err
}

func lookupAddr(addr string, attemptNumber int, lookupTimeout time.Duration, maxAttempts int) (hosts []string, err error) {
	timeout := make(chan error)
	go func() {
		hosts, err = unboundInstance.LookupAddr(addr)
		timeout <- err
	}()

	select {
	case err = <-timeout:
		if err != nil {
			if attemptNumber < maxAttempts {
				return lookupHost(addr, attemptNumber+1, lookupTimeout, maxAttempts)
			}
			syncPrintf("!!!!! failed: max attempts exhausted for addr=%s error=%s\n", addr, err)
		}
	case <-time.After(lookupTimeout * time.Second):
		err = fmt.Errorf("!!!!! error: timed out for \"%v\" after %v seconds", addr, lookupTimeout)
	}
	return hosts, err
}

func lookupTxt(domain string, attemptNumber int, lookupTimeout time.Duration, maxAttempts int) (txts []string, err error) {
	timeout := make(chan error)
	go func() {
		txts, err = unboundInstance.LookupTXT(domain)
		timeout <- err
	}()

	select {
	case err = <-timeout:
		if err != nil {
			if attemptNumber < maxAttempts {
				return lookupTxt(domain, attemptNumber+1, lookupTimeout, maxAttempts)
			}
			syncPrintf("!!!!! failed: max attempts exhausted for domain=%s error=%s\n", domain, err)
		}
	case <-time.After(lookupTimeout * time.Second):
		err = fmt.Errorf("!!!!! error: timed out for \"%v\" after %v seconds", domain, lookupTimeout)
	}
	return txts, err
}

func lookupMX(domain string, attemptNumber int, lookupTimeout time.Duration, maxAttempts int) (mx []string, err error) {
	mxDNS := []*dns.MX{}
	timeout := make(chan error)
	go func() {
		mxDNS, err = unboundInstance.LookupMX(domain)
		timeout <- err
	}()

	select {
	case err = <-timeout:
		if err != nil {
			if attemptNumber < maxAttempts {
				return lookupMX(domain, attemptNumber+1, lookupTimeout, maxAttempts)
			}
			syncPrintf("!!!!! failed: max attempts exhausted for domain=%s error=%s\n", domain, err)
		}
	case <-time.After(lookupTimeout * time.Second):
		err = fmt.Errorf("!!!!! error: timed out for \"%v\" after %v seconds", domain, lookupTimeout)
	}

	for _, mxRec := range mxDNS {
		mx = append(mx, fmt.Sprintf("%s", mxRec))
	}

	return mx, err
}

func lookupCName(domain string, attemptNumber int, lookupTimeout time.Duration, maxAttempts int) (cname string, err error) {
	timeout := make(chan error)
	go func() {
		cname, err = unboundInstance.LookupCNAME(domain)
		timeout <- err
	}()

	select {
	case err = <-timeout:
		if err != nil {
			if attemptNumber < maxAttempts {
				return lookupCName(domain, attemptNumber+1, lookupTimeout, maxAttempts)
			}
			syncPrintf("!!!!! failed: max attempts exhausted for domain=%s error=%s\n", domain, err)
		}
	case <-time.After(lookupTimeout * time.Second):
		err = fmt.Errorf("!!!!! error: timed out for \"%v\" after %v seconds", domain, lookupTimeout)
	}
	return cname, err
}

func resolve(domainStr string, lookupTimeout time.Duration, maxAttempts int) {
	if IPMatch.MatchString(domainStr) {
		hosts, hostsErr := lookupAddr(domainStr, 1, lookupTimeout, maxAttempts)
		ch <- Result{domainStr, "", nil, nil, nil, "", hosts, nil, nil, nil, nil, hostsErr}
	} else {
		addrs, addrErr := lookupHost(domainStr, 1, lookupTimeout, maxAttempts)
		txts, txtErr := lookupTxt(domainStr, 1, lookupTimeout, maxAttempts)
		mx, mxErr := lookupMX(domainStr, 1, lookupTimeout, maxAttempts)
		cName, cNameErr := lookupCName(domainStr, 1, lookupTimeout, maxAttempts)
		ch <- Result{domainStr, domainStr, addrs, txts, mx, cName, nil, addrErr, txtErr, mxErr, cNameErr, nil}
	}
}

func resolveWorker(linkChan chan string, wg *sync.WaitGroup, lookupTimeout time.Duration, maxAttempts int) {
	defer wg.Done()

	for domain := range linkChan {
		resolve(domain, lookupTimeout, maxAttempts)
	}
}

func syncPrintf(msg string, args ...interface{}) {
	outputLock.Lock()
	fmt.Printf(msg, args...)
	_ = os.Stdout.Sync()
	outputLock.Unlock()
}

var logger *slog.Logger

func init() {
	logger = common.WithGroup("dns_utils")

}

func Run(domains []string, resolveConf string, concurrency int, lookupTimeout time.Duration, maxAttempts int, ignoreAddrs []string, ignoreAliases []string) {
	err := unboundInstance.ResolvConf(resolveConf)
	if err != nil {
		logger.Error("unable to start unbound", "err", err.Error(), "resolveConf", resolveConf)
	}
	domainCount := len(domains)
	logger.Debug("resolving domains", "domainCount", domainCount, "workers", concurrency)

	ch = make(chan Result, concurrency)
	tasks := make(chan string, concurrency)

	// Spawn resolveWorker goroutines.
	wg := new(sync.WaitGroup)

	// Adding routines to workgroup and running then.
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go resolveWorker(tasks, wg, lookupTimeout, maxAttempts)
	}

	receiver := func(numDomains int) {
		defer wg.Done()

		i := 0
	Loop:
		for {
			select {
			case result := <-ch:
				output := []string{}

				logger.Debug("domain result", "result", result)

				for _, ip := range result.addresses {
					if slices.Contains(ignoreAddrs, ip) {
						continue
					}
					output = append(output, fmt.Sprintf("%s has address %s\n", result.domain, ip))
				}

				for _, txt := range result.txt {
					output = append(output, fmt.Sprintf("%s has TXT %s\n", result.domain, txt))
				}

				for _, mx := range result.mx {
					output = append(output, fmt.Sprintf("%s has MX %s\n", result.domain, mx))
				}

				for _, host := range result.hosts {
					output = append(output, fmt.Sprintf("%s domain name pointer %s\n", result.domain, host))
				}

				if len(result.cName) > 0 {
					if !slices.Contains(ignoreAliases, result.cName) {
						output = append(output, fmt.Sprintf("%s is an alias for %s\n", result.domain, result.cName))
					}
				}

				syncPrintf(strings.Join(output, ""))
				i++
				if i == numDomains {
					break Loop
				}
			}
		}
	}

	wg.Add(1)
	go receiver(domainCount)

	// Processing all links by spreading them to `free` goroutines
	for _, domain := range domains {
		tasks <- domain
	}

	close(tasks)
	wg.Wait()
}
