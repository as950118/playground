package com.okestro.practice;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.List;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;

public class ParallelHttp {
    public static void main(String[] args) throws Exception {
        HttpClient client = HttpClient.newHttpClient();
        List<String> urls = List.of(
                "https://httpbin.org/delay/1",
                "https://httpbin.org/delay/1",
                "https://httpbin.org/delay/1");

        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            List<Future<Integer>> futures = urls.stream()
                    .map(url -> executor.submit(() -> {
                        var req = HttpRequest.newBuilder(URI.create(url)).build();
                        // 동기 코드 그대로 작성. 블로킹되어도 캐리어 스레드는 반납됨
                        return client.send(req, HttpResponse.BodyHandlers.ofString()).statusCode();
                    }))
                    .toList();

            for (Future<Integer> f : futures) {
                System.out.println("status = " + f.get());
            }
        }
    }
}