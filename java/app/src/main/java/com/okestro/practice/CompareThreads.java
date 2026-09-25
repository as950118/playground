package com.okestro.practice;

import java.time.Duration;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.stream.IntStream;

public class CompareThreads {
    static long run(ExecutorService executor) {
        long start = System.currentTimeMillis();
        try (executor) {
            IntStream.range(0, 1_000).forEach(i ->
                    executor.submit(() -> {
                        Thread.sleep(Duration.ofMillis(500));
                        return i;
                    }));
        }
        return System.currentTimeMillis() - start;
    }

    public static void main(String[] args) {
        System.out.println("platform(100): " + run(Executors.newFixedThreadPool(100)) + "ms");   // ì½ 5000ms
        System.out.println("virtual      : " + run(Executors.newVirtualThreadPerTaskExecutor()) + "ms"); // ì½ 500ms
    }
}