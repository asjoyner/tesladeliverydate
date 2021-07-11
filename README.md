# Tesla Delivery Date

Monitor for changes in the delivery date of a Tesla Reservation Number.

As has [been noted in news articles](https://electrek.co/2021/07/05/i-just-bought-my-very-first-tesla-heres-what-happened/), or as you can [easily tell from a Google search](https://www.google.com/search?q=tesla+delivery+date+keeps+changing), individuals who have ordered a Tesla like to keep a keen eye on when their vehicle might arrive.  They are also often frustrated by how often the Tesla website silently updates with new information.

This small utility will monitor for the delivery date of a Tesla by Reservation Number, and output the latest information to the screen.  It's not a particularly user friendly UI, but it serves my needs.

To use it, first copy the example configuration file to your home directory (at `~/.tesladeliverydate` or any path you provide as `--config`), and edit it to match your account and car.  Then, just start the daemon by running `go run main.go`.

It prints output to the screen like this:

```
$ go run main.go
I0710 22:54:28.362919   26303 main.go:190] === New Delivery Date! ===
I0710 22:54:28.363197   26303 main.go:192] Estimated Delivery: July 17 - July 23
```

It will check for updates on the delivery date once an hour.

