Appfw - Application Firewall Daemon
==================

Small daemon to easily make application path specific rules.
I.e. prevent bruteforcing your login system by:

- Setting a 24hour (daily) limit on the total amount of login attempts;
- Setting a limit of 3 attempts per IP per 24hour;
- Setting a limit of 4 attempts per email per 24hour;

Code example:
```bash
curl 'http://127.0.0.1/fw?query=authlogin&limit=1500&strategy=24h_first'
curl 'http://127.0.0.1/fw?query=authlogin-$ip&limit=3&strategy=24h_last'

curl 'http://127.0.0.1/fw?query=authlogin-$email&limit=4&strategy=24h_last'
curl 'http://127.0.0.1/fw?query=authlogin-$email&limit=4&strategy=24h_last'
curl 'http://127.0.0.1/fw?query=authlogin-$email&limit=4&strategy=24h_last'
curl 'http://127.0.0.1/fw?query=authlogin-$email&limit=4&strategy=24h_last'
< 403 Reject further processing in your app
```

This daemon is just a fancy counter, by adding 'rules' to your website-code
you easily extend your website with a fancy application firewall.
