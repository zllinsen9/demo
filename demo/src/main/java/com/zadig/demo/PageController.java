package com.zadig.demo;

import org.springframework.stereotype.Controller;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.servlet.ModelAndView;

@Controller
public class PageController {

    @RequestMapping("/index")
    public ModelAndView index(){
        return new ModelAndView("index");
    }




    @RequestMapping("/login")
    public ModelAndView login(){
        return new ModelAndView("login");
    }
}
