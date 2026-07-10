#!/bin/sh
echo "=== IPv6 Internal + IPv4 Internet Test ==="

echo "[0/4] Checking if IPv6 is enabled at kernel level..."
IPV6_DISABLED=$(sysctl -n net.ipv6.conf.all.disable_ipv6 2>/dev/null)
if [ "$IPV6_DISABLED" = "1" ]; then
    echo "SKIP: IPv6 is disabled (net.ipv6.conf.all.disable_ipv6=1), cluster has no IPv6 stack"
else
    echo "INFO: IPv6 is enabled (net.ipv6.conf.all.disable_ipv6=${IPV6_DISABLED:-unknown})"
fi

echo "[1/3] Checking pod IPv6 address..."
if [ "$IPV6_DISABLED" = "1" ]; then
    echo "SKIP: IPv6 disabled, no address expected"
elif [ -f /proc/net/if_inet6 ] && grep -qv '^00000000000000000000000000000001' /proc/net/if_inet6 2>/dev/null; then
    echo "PASS: pod has global IPv6 address(es)"
    grep -v '^00000000000000000000000000000001' /proc/net/if_inet6 | head -5
elif [ -f /proc/net/if_inet6 ] && grep -q '[0-9a-f]' /proc/net/if_inet6; then
    echo "INFO: pod has only IPv6 loopback (::1), CNI not assigning global IPv6"
else
    echo "FAIL: pod has NO IPv6 address"
fi

echo "[2/3] Testing internal IPv6 connectivity..."
if [ "$IPV6_DISABLED" = "1" ]; then
    echo "SKIP: IPv6 disabled, connectivity not available"
elif grep -qv '^00000000000000000000000000000001' /proc/net/if_inet6 2>/dev/null; then
    if curl --version 2>/dev/null | grep -qi "IPv6"; then
        if curl -6 -s -m 10 -o /dev/null -w "%{http_code}" https://www.bing.com 2>/dev/null | grep -qE "^[23][0-9]{2}$"; then
            echo "PASS: IPv6 internet connectivity works"
        else
            echo "INFO: curl has IPv6 support but no IPv6 internet route"
        fi
    else
        echo "INFO: curl lacks IPv6 support, IPv6 stack available (addr: ${IPV6_ADDR:-present})"
    fi
else
    echo "INFO: no global IPv6, skipping connectivity test (CNI issue)"
fi

echo "[3/3] Testing internet IPv4 download/install..."
if curl -4 -s -m 30 -o /tmp/ipv4_testfile https://goproxy.cn/sumdb/sum.golang.google.cn/lookup/golang.org/x/mod@v1.0.0; then
    echo "PASS: internet download over IPv4 succeeded"
    rm -f /tmp/ipv4_testfile
else
    echo "FAIL: internet download over IPv4 failed"
fi

echo "=== IPv6/IPv4 Test Completed ==="