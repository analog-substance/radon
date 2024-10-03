package cmd

import (
	"bufio"
	_ "embed"
	"fmt"
	"github.com/analog-substance/radon/pkg/common"
	"github.com/analog-substance/radon/pkg/dns_utils"
	"github.com/analog-substance/radon/pkg/permute"
	"github.com/asaskevich/govalidator"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"log"
	"log/slog"
	"os"
	"path/filepath"
)

var (
	//go:embed fast-resolv.conf
	defaultConf []byte
	debug       = false
	logger      *slog.Logger
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "radon",
	Short: "Fast DNS resolver and bruteforce tool",
	Long: `Fast DNS resolver and bruteforce tool. For example:

Resolve domains in file
	radon -d domains.txt

Generate permutations and resolve them
	radon -pd domains.txt
`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	PreRun: func(cmd *cobra.Command, args []string) {
		if debug {
			common.LogLevel(slog.LevelDebug)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		domainsFile, _ := cmd.Flags().GetString("domains-file")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		resolveConfFile, _ := cmd.Flags().GetString("resolve-conf")
		lookupTimeout, _ := cmd.Flags().GetDuration("lookup-timeout")
		maxAttempts, _ := cmd.Flags().GetInt("max-attempts")
		ignoreAddrs, _ := cmd.Flags().GetStringSlice("ignore-addr")
		ignoreAliases, _ := cmd.Flags().GetStringSlice("ignore-alias")
		shouldPermute, _ := cmd.Flags().GetBool("permute")
		shouldDoRandom, _ := cmd.Flags().GetBool("invoke-random")

		domains := getDomains(domainsFile)

		domainCount := len(domains)
		if domainCount == 0 {
			fmt.Println("[!] No Domains found")
			return
		}

		var newDomains []string

		if shouldPermute {

			for _, targetDomain := range domains {
				newDomains = append(newDomains, permute.IncrementInts(targetDomain)...)
			}

			for _, targetDomain := range newDomains {
				for _, replacements := range permute.ReplacementGroups {
					newDomains = append(newDomains, permute.ExtrapolateNewDomains(targetDomain, replacements)...)
				}
			}

		}

		if shouldDoRandom {
			newDomains = append(newDomains, permute.RandomBrute(domains)...)
		}

		logger.Debug("input domain count", "count", domainCount)
		uniqueNewDomains := map[string]bool{}
		for _, targetDomain := range domains {
			uniqueNewDomains[targetDomain] = true
		}

		uniqueCount := len(uniqueNewDomains)
		logger.Debug("unique domain count", "count", uniqueCount)
		for _, targetDomain := range newDomains {
			uniqueNewDomains[targetDomain] = true
		}

		totalDomainCount := len(uniqueNewDomains)
		additionalDomains := totalDomainCount - uniqueCount
		logger.Debug("additional permuted domains count", "count", additionalDomains)
		logger.Debug("total domain count", "count", totalDomainCount)

		var uniqDomainSlice []string
		for uniqueNewDomain := range uniqueNewDomains {
			uniqDomainSlice = append(uniqDomainSlice, uniqueNewDomain)
		}

		dns_utils.Run(uniqDomainSlice, resolveConfFile, concurrency, lookupTimeout, maxAttempts, ignoreAddrs, ignoreAliases)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	logger = common.Logger()

	home, err := homedir.Dir()
	if err != nil {
		log.Fatal(err)
	}

	confDir := filepath.Join(home, ".radon")
	if !exists(confDir) {
		_ = os.MkdirAll(confDir, 0755)
	}

	defaultResolvConf := filepath.Join(confDir, "resolv.conf")
	if !exists(defaultResolvConf) {
		_ = os.WriteFile(defaultResolvConf, defaultConf, 0644)
	}
	RootCmd.Flags().IntP("concurrency", "c", 100, "Concurrent lookups")
	RootCmd.Flags().StringP("domains-file", "d", "domains.txt", "path to file with domains to lookup")
	RootCmd.Flags().StringP("resolve-conf", "r", defaultResolvConf, "Path resolve.conf")
	RootCmd.Flags().StringSlice("ignore-addr", []string{}, "Ignore hosts that resolve to specific ip(s)")
	RootCmd.Flags().StringSlice("ignore-alias", []string{}, "Ignore hosts that resolve to specific alias(es)")

	RootCmd.Flags().DurationP("lookup-timeout", "t", 10, "Lookup timeout")
	RootCmd.Flags().IntP("max-attempts", "a", 4, "Number of failed attempts before we give up")
	RootCmd.Flags().BoolVar(&debug, "debug", false, "Display debug information")
	RootCmd.Flags().Bool("invoke-random", false, "Jumble used tokens to make new subdomains.")
	RootCmd.Flags().Bool("permute", false, "Permute domains.")

}

func exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func getDomains(domainsFile string) []string {
	var domains []string

	var scanner *bufio.Scanner

	if domainsFile == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		file, err := os.Open(domainsFile)
		if err != nil {
			log.Fatal(err)
		}

		defer func(file *os.File) {
			_ = file.Close()
		}(file)
		scanner = bufio.NewScanner(file)
	}

	for scanner.Scan() {
		domain := scanner.Text()
		if dns_utils.IPMatch.MatchString(domain) {
			domains = append(domains, domain)
			continue
		}

		domain = dns_utils.DomainCleanerRe.ReplaceAllString(domain, "$2")
		if govalidator.IsDNSName(domain) {
			domains = append(domains, domain)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return domains
}
