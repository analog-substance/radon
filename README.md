# radon
Fast DNS resolver and bruteforce tool

## Usage

Radon will return output in a format similar to the `host` command.

### Resolve domains

```bash
radon -d my-domains.txt
```

### Bruteforce Domains

```bash
radon --invoke-random --permute -d my-domains.txt
```

`--invoke-random` will analyze domains for tokens, then replace tokens with other discovered tokens.
```txt
asd-test-thing.example.com
```
Becomes:
```txt
asd-asd-thing.example.com
asd-asd-asd.example.com
test-asd-thing.example.com
test-test-thing.example.com
....
```

`--permute` will attempt to find common patterns like aws regions, country codes, or numbers and replace them with other variants.

```txt
example1.example.com
```

Becomes
```txt
example0.example.com
example1.example.com
example2.example.com
example3.example.com
example4.example.com
example5.example.com
example6.example.com
example7.example.com
example8.example.com
example9.example.com
```