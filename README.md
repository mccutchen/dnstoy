# dnstoy

Working through [Julia Evans][]'s [Implement DNS in a Weekend][] project.

## Development/testing

```bash
# build
make

# resolve default set of domains, w/ debug output
./bin/dnstoy -debug

# resolve specific domain
./bin/dnstoy www.example.com

# run tests
make test
```

[Julia Evans]: https://twitter.com/b0rk
[Implement DNS in a Weekend]: https://implement-dns.wizardzines.com/
