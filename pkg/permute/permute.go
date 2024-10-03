package permute

import (
	"fmt"
	"golang.org/x/net/publicsuffix"
	"log"
	"regexp"
	"sort"
	"strings"
)

type Domain struct {
	PublicSuffix *Domain
	Parent       *Domain `json:"-"`
	Value        string
	SubDomains   map[string]*Domain
}

func (d *Domain) FQDN() string {
	var o []string
	cd := d
	for {
		o = append(o, cd.Value)
		cd = cd.Parent
		if cd == nil {
			break
		}
	}
	return strings.Join(o, ".")
}

func sortedKeysForMapStringInt(target map[string]int) []string {
	targetKeys := make([]string, 0, len(target))
	for key := range target {
		targetKeys = append(targetKeys, key)
	}
	sort.SliceStable(targetKeys, func(i, j int) bool {
		return target[targetKeys[i]] > target[targetKeys[j]]
	})
	return targetKeys
}

func (d *Domain) SubData() map[string]int {
	o := map[string]int{}
	cd := d
	for _, subDomain := range cd.SubDomains {
		o[subDomain.Value]++
		for s, c := range subDomain.SubData() {
			o[s] += c
		}
	}
	return o
}

func (d *Domain) AddSubDomain(subName string) *Domain {

	fqdn := d.FQDN()
	if fqdn == subName {
		return d
	}
	if strings.Contains(subName, ".") {
		split := strings.Split(strings.TrimSuffix(subName, "."+fqdn), ".")
		cd := d
		i := len(split) - 1
		for ; i >= 0; i-- {
			cd = cd.AddSubDomain(split[i])
		}
		return cd
	}

	_, ok := d.SubDomains[subName]
	if !ok {
		d.SubDomains[subName] = &Domain{
			Parent:       d,
			Value:        subName,
			SubDomains:   make(map[string]*Domain),
			PublicSuffix: d.PublicSuffix,
		}
	}
	return d.SubDomains[subName]
}

var domainMap = map[string]*Domain{}

func newDomain(name string) *Domain {
	ps, err := publicsuffix.EffectiveTLDPlusOne(name)
	if err != nil {
		log.Fatal(err)
	}

	_, ok := domainMap[ps]
	if !ok {
		domainMap[ps] = &Domain{
			Value:      ps,
			SubDomains: make(map[string]*Domain),
		}
	}

	return domainMap[ps].AddSubDomain(name)
}

func IncrementInts(targetDomain string) (newDomains []string) {
	realLimit := 255

	intMatch := regexp.MustCompile(`([0-9]+)`)
	matches := intMatch.FindAllStringSubmatchIndex(targetDomain, -1)

	for _, match := range matches {
		pad := match[1] - match[0]
		limit := 1
		for i := 0; i < pad; i++ {
			limit *= 10
		}

		tmpRE := regexp.MustCompile(fmt.Sprintf("^(.{%d})([0-9]+)", match[0]))

		var myDomains []string
		for i := 0; i < limit; i++ {
			if i > realLimit {
				break
			}

			myDomains = append(myDomains, tmpRE.ReplaceAllString(targetDomain, fmt.Sprintf("${1}%0*d", pad, i)))
			for _, domain := range newDomains {
				myDomains = append(myDomains, tmpRE.ReplaceAllString(domain, fmt.Sprintf("${1}%0*d", pad, i)))
			}
		}
		newDomains = append(newDomains, myDomains...)
	}

	return newDomains
}

func ExtrapolateNewDomains(targetDomain string, tokens []string) (newDomains []string) {
	ps, err := publicsuffix.EffectiveTLDPlusOne(targetDomain)
	if err != nil {
		//log.Fatal(err)
		return newDomains
	}

	if ps == targetDomain {
		return newDomains
	}

	domain := strings.TrimSuffix(targetDomain, "."+ps)
	for _, token := range tokens {
		checkRe := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(token)))
		if checkRe.MatchString(domain) {
			for _, rplToken := range tokens {
				if rplToken != token {
					newDomains = append(newDomains, checkRe.ReplaceAllString(domain, rplToken)+"."+ps)
				}
			}
		}
	}

	return newDomains
}

func RandomBrute(domains []string) []string {

	for _, domain := range domains {
		newDomain(domain)
	}

	subdomainMap := make(map[string]int)

	for _, domain := range domainMap {

		subData := domain.SubData()
		for entry, count := range subData {
			subdomainMap[entry] += count
		}
	}

	tokens := map[string]int{}
	re := regexp.MustCompile(`([a-z0-9]{2,100}\b)*`)

	for entry := range subdomainMap {

		// breakdown subdomain into tokens and count how many times they are used.
		matches := re.FindAllString(entry, -1)
		for _, match := range matches {
			if len(match) > 0 {
				tokens[match]++
			}
		}

		// attempt to find larger tokens like dumb-mobile-test-1 starting from the beginning
		// appending characters
		tmp := map[string]int{}
		for i := 1; i <= len(entry); i++ {
			for entryCmp := range subdomainMap {
				if strings.Contains(entryCmp, entry[0:i]) {
					tmp[entry[0:i]]++
				}
			}

			_, ok := tmp[entry[0:i]]
			if !ok {
				break
			}

		}

		largestKey := ""
		for key := range tmp {
			if len(key) > len(largestKey) {
				largestKey = key
			}
		}
		if largestKey != "" {
			tokens[largestKey] += tmp[largestKey]
		}

		// attempt to find larger tokens like dumb-mobile-test-1 starting from the beginning
		// removing preceding chars
		tmp = map[string]int{}
		for i := len(entry); i >= 0; i-- {
			for entryCmp := range subdomainMap {
				if strings.Contains(entryCmp, entry[i:]) {
					tmp[entry[i:]]++
				}
			}

			_, ok := tmp[entry[i:]]
			if !ok {
				break
			}

		}

		largestKey = ""
		for key := range tmp {
			if len(key) > len(largestKey) {
				largestKey = key
			}
		}
		if largestKey != "" {
			tokens[largestKey] += tmp[largestKey]
		}
	}

	var newDomains []string
	sortedTokenKeys := sortedKeysForMapStringInt(tokens)
	for _, token := range sortedTokenKeys {
		var matched []string
		checkRe := regexp.MustCompile(fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(token)))
		for _, domain := range domains {
			if checkRe.MatchString(domain) {
				matched = append(matched, domain)
			}
		}

		if len(matched) > 1 {
			for _, otherToken := range sortedTokenKeys {
				for _, match := range matched {
					rootDomain, err := publicsuffix.EffectiveTLDPlusOne(match)
					if err != nil {
						log.Fatal(err)
					}
					if rootDomain == match {
						continue
					}
					suffix := fmt.Sprintf(".%s", rootDomain)
					targetDomain := strings.TrimSuffix(match, suffix)

					newDomains = append(newDomains, fmt.Sprintf("%s%s", checkRe.ReplaceAllString(targetDomain, otherToken), suffix))
				}
			}
		}
	}

	return newDomains
}

var ReplacementGroups = map[string][]string{
	"App functions": {
		"console",
		"panel",
		"admin",
		"portal",
	},
	"Terrible version management": {
		"new",
		"old",
		"backup",
		"newer",
		"newest",
		"next",
	},
	"Application Things": {
		"app",
		"api",
		"ui",
		"graphql",
	},
	"Marketing Sites": {
		"www",
		"www1",
		"www2",
		"www3",
		"support",
		"partners",
		"community",
		"helpdesk",
		"status",
	},
	"Environments": {
		"prod",
		"prd",
		"dev",
		"stage",
		"stg",
		"staging",
		"test",
		"eng",
		"ops",
		"devops",
	},
	"AWS Regions": {
		"af-south-1",
		"ap-east-1",
		"ap-northeast-1",
		"ap-northeast-2",
		"ap-northeast-3",
		"ap-south-1",
		"ap-south-2",
		"ap-southeast-1",
		"ap-southeast-2",
		"ap-southeast-3",
		"ap-southeast-4",
		"ap-southeast-5",
		"ca-central-1",
		"ca-west-1",
		"eu-central-1",
		"eu-central-2",
		"eu-north-1",
		"eu-south-1",
		"eu-south-2",
		"eu-west-1",
		"eu-west-2",
		"eu-west-3",
		"il-central-1",
		"me-central-1",
		"me-south-1",
		"sa-east-1",
		"us-east-1",
		"us-east-2",
		"us-west-1",
		"us-west-2",
	},
	"Country Codes": {
		"af",
		"al",
		"dz",
		"as",
		"ad",
		"ao",
		"ai",
		"aq",
		"ag",
		"ar",
		"am",
		"aw",
		"au",
		"at",
		"az",
		"bs",
		"bh",
		"bd",
		"bb",
		"by",
		"be",
		"bz",
		"bj",
		"bm",
		"bt",
		"bo",
		"bq",
		"ba",
		"bw",
		"bv",
		"br",
		"io",
		"bn",
		"bg",
		"bf",
		"bi",
		"cv",
		"kh",
		"cm",
		"ca",
		"ky",
		"cf",
		"td",
		"cl",
		"cn",
		"cx",
		"cc",
		"co",
		"km",
		"cd",
		"cg",
		"ck",
		"cr",
		"hr",
		"cu",
		"cw",
		"cy",
		"cz",
		"ci",
		"dk",
		"dj",
		"dm",
		"do",
		"ec",
		"eg",
		"sv",
		"gq",
		"er",
		"ee",
		"sz",
		"et",
		"fk",
		"fo",
		"fj",
		"fi",
		"fr",
		"gf",
		"pf",
		"tf",
		"ga",
		"gm",
		"ge",
		"de",
		"gh",
		"gi",
		"gr",
		"gl",
		"gd",
		"gp",
		"gu",
		"gt",
		"gg",
		"gn",
		"gw",
		"gy",
		"ht",
		"hm",
		"va",
		"hn",
		"hk",
		"hu",
		"is",
		"in",
		"id",
		"ir",
		"iq",
		"ie",
		"im",
		"il",
		"it",
		"jm",
		"jp",
		"je",
		"jo",
		"kz",
		"ke",
		"ki",
		"kp",
		"kr",
		"kw",
		"kg",
		"la",
		"lv",
		"lb",
		"ls",
		"lr",
		"ly",
		"li",
		"lt",
		"lu",
		"mo",
		"mg",
		"mw",
		"my",
		"mv",
		"ml",
		"mt",
		"mh",
		"mq",
		"mr",
		"mu",
		"yt",
		"mx",
		"fm",
		"md",
		"mc",
		"mn",
		"me",
		"ms",
		"ma",
		"mz",
		"mm",
		"na",
		"nr",
		"np",
		"nl",
		"nc",
		"nz",
		"ni",
		"ne",
		"ng",
		"nu",
		"nf",
		"mp",
		"no",
		"om",
		"pk",
		"pw",
		"ps",
		"pa",
		"pg",
		"py",
		"pe",
		"ph",
		"pn",
		"pl",
		"pt",
		"pr",
		"qa",
		"mk",
		"ro",
		"ru",
		"rw",
		"re",
		"bl",
		"sh",
		"kn",
		"lc",
		"mf",
		"pm",
		"vc",
		"ws",
		"sm",
		"st",
		"sa",
		"sn",
		"rs",
		"sc",
		"sl",
		"sg",
		"sx",
		"sk",
		"si",
		"sb",
		"so",
		"za",
		"gs",
		"ss",
		"es",
		"lk",
		"sd",
		"sr",
		"sj",
		"se",
		"ch",
		"sy",
		"tw",
		"tj",
		"tz",
		"th",
		"tl",
		"tg",
		"tk",
		"to",
		"tt",
		"tn",
		"tr",
		"tm",
		"tc",
		"tv",
		"ug",
		"ua",
		"ae",
		"gb",
		"um",
		"us",
		"uy",
		"uz",
		"vu",
		"ve",
		"vn",
		"vg",
		"vi",
		"wf",
		"eh",
		"ye",
		"zm",
		"zw",
		"ax",
	},
}
