<?php
/**
 * Abuse protection (application firewall)
 * this uses mpdroog/core - https://github.com/mpdroog/core
 * (but without is easy if you copy/paste specific funcs out of the lib)
 */
use core\Env;
use core\Helper;
use core\Res;

/** Force 1hour wait after last attempt (Useful for user specific limits) */
const STRATEGY_1H_WAIT = "STRATEGY_1H_WAIT";
/** Force 24hour wait after last attempt (Useful for user specific limits) */
const STRATEGY_24H_WAIT = "STRATEGY_24H_WAIT";
/** Force 24hour wait from the first attempt (Useful for generic limits i.e. 100 lostpass a day) */
const STRATEGY_24H_ADD = "STRATEGY_24H_ADD";

/**
 * Brute-force protection by key.
 *
 * Small abuse counter that uses a unique value (IP/email/memberid) or daily count
 * to limit endpoint calling.
 * This is a useful feature to prevent bruteforce attacks by letting an endpoint
 * determine multiple limits on a daily scale.
 */
class Abuse
{
    private static $base = "http://appfw:1337";
    private static $whitelisted;

    /** cURL handler (for HTTP KeepAlive) */
    public static $ch = null;

    /** Prepare state */
    public static function init()
    {
        $cfg = Helper::config("abuse");
        self::$base = $cfg["server"];
        self::$whitelisted = in_array(Env::ip(), $cfg["whitelist"]);

        self::$ch = curl_init();
        if (self::$ch === false) {
            user_error("Abuse::curl_init fail");
        }
    }

    /**
     * Look at counter $key and block if limit reached.
     * DevNote: Func becomes blocking HTTP(403) that stopts processing on limit reached
     */
    public static function check($key)
    {
        $ch = self::$ch;
        $opts = [
            CURLOPT_URL => sprintf("%s/check?key=%s", self::$base, rawurlencode($key)),
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
            return;
        }
        $http = curl_getinfo($ch, CURLINFO_HTTP_CODE);

        // Ensure everything went right on appfw-side
        if ($http === 403) {
            header("Ratelimit-Key: $key");
            header("Content-Type: text/plain");
            http_response_code(403);
            // TODO: Your own pretty error response here?
            echo "Err, too many requests.";
            exit;
        } else if ($http !== 204) {
            error_log("WARN: Abuse::incr curl_exec($http)=" . $res);
            return;
        }
    }

    /**
     * Increase counter on $key until $maxAttempts
     * DevNote: Func becomes blocking HTTP(403) that stopts processing on limit reached
     */
    public static function incr($key, $maxAttempts=60, $strategy = STRATEGY_24H_WAIT)
    {
        $now = time();
        if (! in_array($strategy, [STRATEGY_24H_WAIT, STRATEGY_24H_ADD, STRATEGY_1H_WAIT])) {
            user_error("DevErr: Invalid Abuse strategy=$strategy");
        }

        if (self::$whitelisted) {
            return;
        }
        $strat = ($strategy === STRATEGY_24H_WAIT) ? "24UPDATE" : "24ADD";
        $strat = ($strategy === STRATEGY_1H_WAIT) ? "1UPDATE" : $strat; // TODO: dirty..

        $ch = self::$ch;
        $opts = [
            CURLOPT_URL => sprintf("%s/limit?key=%s&max=%d&strategy=%s", self::$base, rawurlencode($key), $maxAttempts, $strat),
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
            return;
        }
        $http = curl_getinfo($ch, CURLINFO_HTTP_CODE);

        // Ensure everything went right on appfw-side
        if ($http === 403) {
            header("Ratelimit-Key: $key");
            header("Content-Type: text/plain");
            http_response_code(403);
            // TODO: Your own pretty error response here?
            echo "Err, too many requests.";
            exit;
        } else if ($http !== 204) {
            error_log("WARN: Abuse::incr curl_exec($http)=" . $res);
            return;
        }
    }
}
