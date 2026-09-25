package com.okestro.practice;

public class VirtualThreadBasic {
    public static void main(String[] args) throws InterruptedException {
        // Virtual Thread 1: Basic start
        Thread vt1 = Thread.ofVirtual().start(() ->
                System.out.println("Hello from " + Thread.currentThread()));

        // Virtual Thread 2: Check if it's a virtual thread
        Thread vt2 = Thread.startVirtualThread(() ->
                System.out.println("isVirtual = " + Thread.currentThread().isVirtual()));

        // Virtual Thread 3: Name the thread
        Thread vt3 = Thread.ofVirtual().name("worker-", 0).unstarted(() ->
                System.out.println("name = " + Thread.currentThread().getName()));
        vt3.start();

        vt1.join();
        vt2.join();
        vt3.join();
    }
}