<?php
/**
 * Admin for Appfw-daemon.
 * A small Go-daemon on the same server that
 * stored a counter to prevent stuff like bruteforcing.
 */

// begin
$timezone = "Europe/Amsterdam";
const BASE = "http://127.0.0.1:1337";
const APIKEY = "SECRET_KEY_HERE";
$ch = curl_init();
if ($ch === false) {
    user_error("Abuse::curl_init fail");
}
if (date_default_timezone_set($timezone) === false) {
    user_error("set_timezone($timezone) failed");
}

function dump() {
    global $ch;

    $opts = [
        CURLOPT_URL => sprintf("%s/memory?apikey=%s", BASE, rawurlencode(APIKEY)),
        CURLOPT_HTTPHEADER => ['Accept: application/json'],
        CURLOPT_CUSTOMREQUEST => "GET",
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HEADER => false
    ];
    if (false === curl_setopt_array($ch, $opts)) {
        user_error("curl_setopt_array failed?");
    }

    $res = curl_exec($ch);
    if ($res === false) {
        die("CURLERR=" . curl_error($ch));
    }
    $http = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    if ($http !== 200) {
        var_dump($res);
        die("ERR HTTP=$http");
    }
    $json = json_decode($res, true);
    if (! is_array($json)) {
        var_dump($res);
        die("ERR, res not JSON?");
    }
    return $json;
}
function clear($query) {
    global $ch;

    $opts = [
        CURLOPT_URL => sprintf("%s/clear?pattern=%s&apikey=%s", BASE, rawurlencode($query), rawurlencode(APIKEY)),
        CURLOPT_HTTPHEADER => ['Accept: application/json'],
        CURLOPT_CUSTOMREQUEST => "GET",
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HEADER => true
    ];
    if (false === curl_setopt_array($ch, $opts)) {
        user_error("curl_setopt_array failed?");
    }

    $res = curl_exec($ch);
    if ($res === false) {
        die("CURLERR=" . curl_error($ch));
    }
    $http = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    if ($http !== 204) {
        var_dump($res);
        die("ERR");
    }

    // Ugly header parsing to get our affect counter
    list($headers, $body) = explode("\r\n\r\n", $res, 2);
    $affect = null;
    foreach (explode("\r\n", $headers) as $hdr) {
        $kv = explode(":", $hdr, 2);
        if (count($kv) < 2) continue;
        list($key, $value) = $kv;

        if ($key === "X-Affect") {
            $affect = $value;
            break;
        }
    }
    return $affect;
}
function cleanup() {
    global $ch;

    $opts = [
        CURLOPT_URL => sprintf("%s/cleanup?apikey=%s", BASE, rawurlencode(APIKEY)),
        CURLOPT_HTTPHEADER => ['Accept: application/json'],
        CURLOPT_CUSTOMREQUEST => "GET",
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_HEADER => true
    ];
    if (false === curl_setopt_array($ch, $opts)) {
        user_error("curl_setopt_array failed?");
    }

    $res = curl_exec($ch);
    if ($res === false) {
        die("CURLERR=" . curl_error($ch));
    }
    $http = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    if ($http !== 204) {
        var_dump($res);
        die("ERR");
    }
}
// end

$affect = "";
$affect_query = "";
if (isset($_GET["clear"])) {
   $affect = clear("*");
   $affect_query = "*";
}
if (isset($_GET["reset"])) {
   if (strlen($_GET["reset"]) < 5) {
     echo 'Probably invalid IP, rejecting.';
     exit;
   }
   $affect = clear($_GET["reset"]);
   $affect_query = $_GET["reset"];
}
if (isset($_GET["cleanup"])) {
    cleanup();
}

// Small sorting function by value
function cmp($a, $b) {
  if ($a["Value"] === $b["Value"]) return 0;
  return ($a["Value"] < $b["Value"]) ? 1 : -1;
}

$list = dump();
usort($list, "cmp");
?>
<html>
<head>
<title>Application Firewall</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@fortawesome/fontawesome-free@6.1.1/css/fontawesome.min.css">
</head>
<body>
<div class="container-fluid">

<?php
echo sprintf('<h1 class="text-danger"><i class="fa fa-fire"></i> Application Firewall (time in %s)</h1>', $timezone);
if ($affect !== "") {
    echo sprintf('<div class="alert alert-banner my-5"><h3>Cleared %s</h3><p>Affect: %d</p></div>', $affect_query, $affect);
}
echo '<form action=""><div class="row align-items-center"><div class="col-auto"><div class="input-group d-flex"><div class="form-floating"><input id="reset" type="text" class="form-control" name="reset" placeholder="value.contains(key)"><label for="reset">value.contains(bar)</label></div><button class="btn btn-primary">Clear</button></div></div><div class="col-auto">';
echo '<a href="?clear" class="js-warn btn btn-outline-primary" data-title="Are you sure you want to clear all abuse entries?">Clear ALL</a>';
echo '<a href="?cleanup" class="js-warn btn btn-outline-primary" data-title="Are you sure you want to remove expired entries?">Cleanup&Refresh</a>';
echo '</div></div></form>';
echo '<table class="table table-ordered">';
echo '<thead><tr><th>Key</th><th>Count</th><th>Max</th><th><abbr title="TimeToLife, datetime until cleared">TTL</abbr></th></tr></thead>';
foreach ($list as $v) {
    $v["Timestamp"] = date("Y-m-d H:i:s", $v["Timestamp"]);
    // Determine color if it needs extra attention
    $percent = $v["Value"] / $v["Max"] * 100;
    $color = "";

    if ($percent >= 80) {
        $color = "text-warning";
    }
    if ($percent >= 90) {
        $color = "text-danger";
    }
    if ($percent >= 100) {
        $color = "bg-dark text-white";
    }

    echo sprintf("<tr class='%s'><td>", $color);
    echo implode("</td><td>", $v);
    echo "</td></tr>";
}
echo '</table>';

echo '<script type="text/javascript">var $nodes = document.getElementsByClassName("js-warn");
for (var i = 0; i < $nodes.length; i++) {
  $nodes[i].addEventListener("click", function(e) {
    if (!confirm(e.target.dataset.title)) {
      e.preventDefault();
    }
  });
}
</script>';
?>
</div></body></html>
