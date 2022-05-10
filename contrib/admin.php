<?php
/**
 * Admin for Appfw-daemon.
 * A small Go-daemon on the same server that
 * stored a counter to prevent stuff like bruteforcing.
 */

// begin
const BASE = "http://127.0.0.1:1337";
const APIKEY = "APIKEY_HERE_FROM_DAEMON";
// Share $ch between funcs to re-use conn where possible
$ch = curl_init();
if ($ch === false) {
    user_error("Abuse::curl_init fail");
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
        list($key, $value) = explode(":", $hdr, 2);
        if ($key === "X-Affect") {
            $affect = $value;
            break;
        }
    }
    return $affect;
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
<link rel="stylesheet" href="/assets/v2.css?v=xyz">
<link rel="stylesheet" href="/css/font-awesome.min.css?v=001">
</head>
<body>
<div class="container-fluid">

<?php
echo '<h1 class="text-danger"><i class="fa fa-fire"></i> Application Firewall</h1>';
if ($affect !== "") {
    echo sprintf('<div class="alert alert-banner my-5"><h3>Cleared %s</h3><p>Affect: %d</p></div>', $affect_query, $affect);
}
echo '<form action=""><div class="row align-items-center"><div class="col-auto"><div class="input-group d-flex"><div class="form-floating"><input id="reset" type="text" class="form-control" name="reset" placeholder="value.contains(key)"><label for="reset">value.contains(bar)</label></div><button class="btn btn-primary">Clear</button></div></div><div class="col-auto">';
echo '<a href="?clear" class="js-warn btn btn-outline-primary">Clear ALL</a>';
//echo '<a href="?refresh" class="js-warn btn btn-outline-primary">Refresh</a>';
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
        $color = "text-dark";
    }

    echo sprintf("<tr class='%s'><td>", $color);
    echo implode("</td><td>", $v);
    echo "</td></tr>";
}
echo '</table>';

echo '<script type="text/javascript">var $nodes = document.getElementsByClassName("js-warn");
for (var i = 0; i < $nodes.length; i++) {
  $nodes[i].addEventListener("click", function(e) {
    if (!confirm("Are you sure you want to clear all abuse entries?")) {
      e.preventDefault();
    }
  });
}
</script>';
?>
</div></body></html>
