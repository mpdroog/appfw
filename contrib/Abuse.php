<?php
/**
 * Abuse protection (application firewall)
 * this uses mpdroog/core - https://github.com/mpdroog/core
 * (but without is easy if you copy/paste specific funcs out of the lib)
 */

use core\Res;
use core\Env;

/** Force 24hour wait after last attempt (Useful for user specific limits) */
const STRATEGY_24H_WAIT = "STRATEGY_24H_WAIT";
/** Force 24hour wait from the first attempt (Useful for generic limits i.e. 100 lostpass a day) */
const STRATEGY_24H_ADD = "STRATEGY_24H_ADD";

/**
 * Brute-force protection that uses the xsnews_abuselimit table.
 *
 * Small abuse counter that uses a unique value (IP/email/memberid) or daily count
 * to limit endpoint calling.
 * This is a useful feature to prevent bruteforce attacks by letting an endpoint
 * determine multiple limits on a daily scale.
 */
class Abuse
{
    const BASE = "http://127.0.0.1:1337";
    private static $ip;
    private static $whitelisted;

    /** cURL handler (for HTTP KeepAlive) */
    public static $ch = null;

    /** Prepare state */
    public static function init($name = "prod")
    {
        self::$db = Shared::db($name);
        // Pre-process
        self::$ip = Env::ip();
        self::$whitelisted = self::whitelisted(self::$ip);

        self::$ch = curl_init();
        if (self::$ch === false) {
            user_error("Abuse::curl_init fail");
        }
    }

    /** List of IPs that don't get blacklisted  */
    public static function whitelisted($ip)
    {
        return in_array($ip, [
                "1.2.3.4"  // Your IP to exclude
        ]);
    }

    /**
     * Increase counter on $ip until $max
     * causes 403 banned when limit $max reached
     */
    public static function incr($key, $maxAttempts=60, $strategy = STRATEGY_24H_WAIT)
    {
        $now = time();
        if (! in_array($strategy, [STRATEGY_24H_WAIT, STRATEGY_24H_ADD])) {
            user_error("DevErr: Invalid Abuse strategy=$strategy");
        }

        if (self::$whitelisted) {
            return;
        }
        $strat = ($strategy === STRATEGY_24H_WAIT) ? "24UPDATE" : "24ADD";

        $ch = self::$ch;
        $opts = [
            CURLOPT_URL => sprintf("%s/limit?key=%s&max=%d&strategy=%s", self::BASE, $key, $maxAttempts, $strat),
            CURLOPT_HTTPHEADER => ['Accept: application/json'],
            CURLOPT_CUSTOMREQUEST => "GET",
            CURLOPT_RETURNTRANSFER => true
        ];
        if (false === curl_setopt_array($ch, $opts)) {
            user_error("curl_setopt_array failed?");
        }

        $res = curl_exec($ch);
        if ($res === false) {
            error_log("WARN: Abuse::incr curl_exec=" . curl_error($ch));
        }
        $http = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        if ($http !== 204) {
            header("Ratelimit-Key: $key");
            Shared::client_error("Err, too many requests.", [], 403);
            exit;
        }
    }
}
