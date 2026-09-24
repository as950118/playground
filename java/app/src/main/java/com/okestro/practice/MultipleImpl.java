package com.okestro.practice;

public class MultipleImpl implements A, B {
    @Override
    public void hi() {
        System.out.println("C");
    }
    public static void main(String[] args) {
        MultipleImpl multipleImpl = new MultipleImpl();
        multipleImpl.hi();
    }
}
interface A { void hi(); }
interface B { void hi(); }