# go3a

go3a is parser library for [animated ascii art format](https://github.com/asciimoth/3a/).  
[![3a logo](https://github.com/asciimoth/3a/blob/main/logo.webp)](https://github.com/asciimoth/3a/blob/main/logo.3a)

Usage example:
```go
f, _ := os.Open("example.3a")
art, err := go3a.Parse3A(f)
if err != nil {
    log.Fatal(err)
}
fmt.Println(art)
```

## Related links
- [3a format spec](https://github.com/asciimoth/3a)
- [aaa tool](https://github.com/asciimoth/aaa)
- [rs3a](https://github.com/asciimoth/rs3a)
- [py3a](https://github.com/asciimoth/py3a)

## License
This project is licensed under either of

- Apache License, Version 2.0, ([LICENSE-APACHE](LICENSE-APACHE) or [apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0))
- MIT license ([LICENSE-MIT](LICENSE-MIT) or [opensource.org/licenses/MIT](http://opensource.org/licenses/MIT))

at your option.

