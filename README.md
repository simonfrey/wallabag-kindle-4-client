# wallabag kindle 4 client

Allows to directly synchronise your wallabag entries to the Kindle 4 ebook library

**I do not know if this works for other Kindles as well. I only tested it on my Kindle 4 (no touch)**

**Do this all at your own risk. Jailbreaking and running custom (my) software as root on your device can brick it. If 
you don't know what you are doing stop here**

## Requirements

1) Jailbroken Kindle 4 (I used [yifan lu](https://yifan.lu/p/kindle-touch-jailbreak/))
2) [SSH access](https://wiki.mobileread.com/wiki/Kindle4NTHacking#SSH) to your Kindle 
3) [Golang installed](https://golang.org/dl/)

## Build arm binary

1) Copy this repo
```bash
git clone https://github.com/simonfrey/wallabag-kindle-4-client.git
```
2) Build arm binary
```bash
env GOOS=linux GOARCH=arm GOARM=5 go build
```

## Installation

1) Copy binary to your kindle (e.g. with SCP)
```bash
scp wallabag-kindle-4-client root@192.168.15.244:/mnt/base-us/documents
```
2) Test run (With ssh on your kindle)
```bash
> cd /mnt/base-us/documents
> mntroot rw
> ./wallabag-kindle-4-client [username] [password] [clientID] [clientSecret]
```
If that worked you are ready to setup the cronjob

3) Setup cronjob (With ssh on your kindle)

3.1) Edit crontab
```bash
> nano /etc/crontab/root 
```
Add the following line (will run every hour)
```bash
0 */1 * * * (cd /mnt/us/documents/ && (./wallabag-kindle-4-client [username] [password] [clientID] [clientSecret] > wallabag-kindle.log))
```

3.2) Restart cron daemon
```bash
> pkill crond && /usr/sbin/crond -c /etc/crontab/
```


