(module
  (type (;0;) (func (param i32)))
  (type (;1;) (func (param i32 i32 i32) (result i32)))
  (type (;2;) (func (param i32 i32) (result i32)))
  (type (;3;) (func (param i64 i32 i32) (result i32)))
  (type (;4;) (func (param i64 i32 i32 i32) (result i32)))
  (type (;5;) (func (param i64 i32 i32 i32 i32) (result i32)))
  (type (;6;) (func (param i32 i64 i32)))
  (type (;7;) (func (param i32) (result i32)))
  (type (;8;) (func (param i32 i32 i32 i32 i32)))
  (type (;9;) (func (param i32 i64 i32 i64)))
  (type (;10;) (func (param i32 i32 i32)))
  (type (;11;) (func (param i32 i32 i32 i32 i32) (result i32)))
  (type (;12;) (func (param i32 i32)))
  (type (;13;) (func (param i64) (result i32)))
  (type (;14;) (func (param i64) (result i64)))
  (type (;15;) (func (param i64 i64 i64) (result i32)))
  (type (;16;) (func (param i64 i64 i64 i64) (result i32)))
  (type (;17;) (func (param i64 i64) (result i64)))
  (type (;18;) (func))
  (type (;19;) (func (param i64 i32) (result i32)))
  (type (;20;) (func (param i32 i32 i32 i32 i32 i32 i32) (result i32)))
  (type (;21;) (func (param i32 i32 i32 i32)))
  (import "state" "len" (func (;0;) (type 3)))
  (import "state" "get" (func (;1;) (type 4)))
  (import "state" "put" (func (;2;) (type 5)))
  (func (;3;) (type 6) (param i32 i64 i32)
    (local i32 i32 i32 i32 i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 3
    global.set 0
    local.get 2
    i32.load offset=8
    local.set 4
    local.get 2
    i32.load
    local.set 5
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            local.get 5
            local.get 4
            local.get 1
            local.get 5
            local.get 4
            call 0
            local.tee 6
            call 1
            local.tee 4
            i32.const 0
            i32.lt_s
            br_if 0 (;@4;)
            local.get 6
            i32.const -1
            i32.le_s
            br_if 3 (;@1;)
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 6
                    i32.const 7
                    i32.gt_u
                    br_if 0 (;@8;)
                    i32.const 0
                    i32.load8_u offset=1053220
                    drop
                    i32.const 16
                    call 4
                    local.tee 7
                    br_if 1 (;@7;)
                    unreachable
                    unreachable
                  end
                  local.get 0
                  i32.const 7
                  i32.store8
                  local.get 0
                  local.get 4
                  i64.load align=1
                  i64.store offset=8
                  br 1 (;@6;)
                end
                local.get 7
                i32.const 2
                i32.store
                local.get 7
                call 5
                local.get 0
                i32.const 1
                i32.store8
                local.get 6
                i32.eqz
                br_if 1 (;@5;)
              end
              local.get 4
              call 6
            end
            local.get 2
            i32.load offset=4
            br_if 1 (;@3;)
            br 2 (;@2;)
          end
          local.get 0
          i32.const 5
          i32.store8
          local.get 2
          i32.load offset=4
          i32.eqz
          br_if 1 (;@2;)
        end
        local.get 5
        call 6
      end
      local.get 3
      i32.const 16
      i32.add
      global.set 0
      return
    end
    i32.const 1048576
    i32.const 19
    local.get 3
    i32.const 8
    i32.add
    i32.const 1048728
    i32.const 1048696
    call 7
    unreachable)
  (func (;4;) (type 7) (param i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i64)
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                local.get 0
                i32.const 245
                i32.lt_u
                br_if 0 (;@6;)
                i32.const 0
                local.set 1
                local.get 0
                i32.const -65587
                i32.ge_u
                br_if 5 (;@1;)
                local.get 0
                i32.const 11
                i32.add
                local.tee 0
                i32.const -8
                i32.and
                local.set 2
                i32.const 0
                i32.load offset=1053644
                local.tee 3
                i32.eqz
                br_if 4 (;@2;)
                i32.const 0
                local.set 4
                block  ;; label = @7
                  local.get 2
                  i32.const 256
                  i32.lt_u
                  br_if 0 (;@7;)
                  i32.const 31
                  local.set 4
                  local.get 2
                  i32.const 16777215
                  i32.gt_u
                  br_if 0 (;@7;)
                  local.get 2
                  i32.const 6
                  local.get 0
                  i32.const 8
                  i32.shr_u
                  i32.clz
                  local.tee 0
                  i32.sub
                  i32.shr_u
                  i32.const 1
                  i32.and
                  local.get 0
                  i32.const 1
                  i32.shl
                  i32.sub
                  i32.const 62
                  i32.add
                  local.set 4
                end
                i32.const 0
                local.get 2
                i32.sub
                local.set 1
                block  ;; label = @7
                  local.get 4
                  i32.const 2
                  i32.shl
                  i32.const 1053232
                  i32.add
                  i32.load
                  local.tee 5
                  br_if 0 (;@7;)
                  i32.const 0
                  local.set 0
                  i32.const 0
                  local.set 6
                  br 2 (;@5;)
                end
                i32.const 0
                local.set 0
                local.get 2
                i32.const 0
                i32.const 25
                local.get 4
                i32.const 1
                i32.shr_u
                i32.sub
                i32.const 31
                i32.and
                local.get 4
                i32.const 31
                i32.eq
                select
                i32.shl
                local.set 7
                i32.const 0
                local.set 6
                loop  ;; label = @7
                  block  ;; label = @8
                    local.get 5
                    i32.load offset=4
                    i32.const -8
                    i32.and
                    local.tee 8
                    local.get 2
                    i32.lt_u
                    br_if 0 (;@8;)
                    local.get 8
                    local.get 2
                    i32.sub
                    local.tee 8
                    local.get 1
                    i32.ge_u
                    br_if 0 (;@8;)
                    local.get 8
                    local.set 1
                    local.get 5
                    local.set 6
                    local.get 8
                    br_if 0 (;@8;)
                    i32.const 0
                    local.set 1
                    local.get 5
                    local.set 6
                    local.get 5
                    local.set 0
                    br 4 (;@4;)
                  end
                  local.get 5
                  i32.const 20
                  i32.add
                  i32.load
                  local.tee 8
                  local.get 0
                  local.get 8
                  local.get 5
                  local.get 7
                  i32.const 29
                  i32.shr_u
                  i32.const 4
                  i32.and
                  i32.add
                  i32.const 16
                  i32.add
                  i32.load
                  local.tee 5
                  i32.ne
                  select
                  local.get 0
                  local.get 8
                  select
                  local.set 0
                  local.get 7
                  i32.const 1
                  i32.shl
                  local.set 7
                  local.get 5
                  i32.eqz
                  br_if 2 (;@5;)
                  br 0 (;@7;)
                end
              end
              block  ;; label = @6
                i32.const 0
                i32.load offset=1053640
                local.tee 7
                i32.const 16
                local.get 0
                i32.const 11
                i32.add
                i32.const -8
                i32.and
                local.get 0
                i32.const 11
                i32.lt_u
                select
                local.tee 2
                i32.const 3
                i32.shr_u
                local.tee 1
                i32.shr_u
                local.tee 0
                i32.const 3
                i32.and
                i32.eqz
                br_if 0 (;@6;)
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 0
                    i32.const -1
                    i32.xor
                    i32.const 1
                    i32.and
                    local.get 1
                    i32.add
                    local.tee 2
                    i32.const 3
                    i32.shl
                    local.tee 5
                    i32.const 1053384
                    i32.add
                    i32.load
                    local.tee 0
                    i32.const 8
                    i32.add
                    local.tee 6
                    i32.load
                    local.tee 1
                    local.get 5
                    i32.const 1053376
                    i32.add
                    local.tee 5
                    i32.eq
                    br_if 0 (;@8;)
                    local.get 1
                    local.get 5
                    i32.store offset=12
                    local.get 5
                    local.get 1
                    i32.store offset=8
                    br 1 (;@7;)
                  end
                  i32.const 0
                  local.get 7
                  i32.const -2
                  local.get 2
                  i32.rotl
                  i32.and
                  i32.store offset=1053640
                end
                local.get 0
                local.get 2
                i32.const 3
                i32.shl
                local.tee 2
                i32.const 3
                i32.or
                i32.store offset=4
                local.get 0
                local.get 2
                i32.add
                local.tee 0
                local.get 0
                i32.load offset=4
                i32.const 1
                i32.or
                i32.store offset=4
                local.get 6
                return
              end
              local.get 2
              i32.const 0
              i32.load offset=1053648
              i32.le_u
              br_if 3 (;@2;)
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        block  ;; label = @11
                          block  ;; label = @12
                            local.get 0
                            br_if 0 (;@12;)
                            i32.const 0
                            i32.load offset=1053644
                            local.tee 0
                            i32.eqz
                            br_if 10 (;@2;)
                            local.get 0
                            i32.const 0
                            local.get 0
                            i32.sub
                            i32.and
                            i32.ctz
                            i32.const 2
                            i32.shl
                            i32.const 1053232
                            i32.add
                            i32.load
                            local.tee 6
                            i32.load offset=4
                            i32.const -8
                            i32.and
                            local.set 1
                            block  ;; label = @13
                              local.get 6
                              i32.load offset=16
                              local.tee 0
                              br_if 0 (;@13;)
                              local.get 6
                              i32.const 20
                              i32.add
                              i32.load
                              local.set 0
                            end
                            local.get 1
                            local.get 2
                            i32.sub
                            local.set 5
                            block  ;; label = @13
                              local.get 0
                              i32.eqz
                              br_if 0 (;@13;)
                              loop  ;; label = @14
                                local.get 0
                                i32.load offset=4
                                i32.const -8
                                i32.and
                                local.get 2
                                i32.sub
                                local.tee 8
                                local.get 5
                                i32.lt_u
                                local.set 7
                                block  ;; label = @15
                                  local.get 0
                                  i32.load offset=16
                                  local.tee 1
                                  br_if 0 (;@15;)
                                  local.get 0
                                  i32.const 20
                                  i32.add
                                  i32.load
                                  local.set 1
                                end
                                local.get 8
                                local.get 5
                                local.get 7
                                select
                                local.set 5
                                local.get 0
                                local.get 6
                                local.get 7
                                select
                                local.set 6
                                local.get 1
                                local.set 0
                                local.get 1
                                br_if 0 (;@14;)
                              end
                            end
                            local.get 6
                            call 27
                            local.get 5
                            i32.const 16
                            i32.lt_u
                            br_if 2 (;@10;)
                            local.get 6
                            local.get 2
                            i32.const 3
                            i32.or
                            i32.store offset=4
                            local.get 6
                            local.get 2
                            i32.add
                            local.tee 2
                            local.get 5
                            i32.const 1
                            i32.or
                            i32.store offset=4
                            local.get 2
                            local.get 5
                            i32.add
                            local.get 5
                            i32.store
                            i32.const 0
                            i32.load offset=1053648
                            local.tee 7
                            br_if 1 (;@11;)
                            br 5 (;@7;)
                          end
                          block  ;; label = @12
                            block  ;; label = @13
                              i32.const 2
                              local.get 1
                              i32.const 31
                              i32.and
                              local.tee 1
                              i32.shl
                              local.tee 5
                              i32.const 0
                              local.get 5
                              i32.sub
                              i32.or
                              local.get 0
                              local.get 1
                              i32.shl
                              i32.and
                              local.tee 0
                              i32.const 0
                              local.get 0
                              i32.sub
                              i32.and
                              i32.ctz
                              local.tee 1
                              i32.const 3
                              i32.shl
                              local.tee 6
                              i32.const 1053384
                              i32.add
                              i32.load
                              local.tee 0
                              i32.const 8
                              i32.add
                              local.tee 8
                              i32.load
                              local.tee 5
                              local.get 6
                              i32.const 1053376
                              i32.add
                              local.tee 6
                              i32.eq
                              br_if 0 (;@13;)
                              local.get 5
                              local.get 6
                              i32.store offset=12
                              local.get 6
                              local.get 5
                              i32.store offset=8
                              br 1 (;@12;)
                            end
                            i32.const 0
                            local.get 7
                            i32.const -2
                            local.get 1
                            i32.rotl
                            i32.and
                            i32.store offset=1053640
                          end
                          local.get 0
                          local.get 2
                          i32.const 3
                          i32.or
                          i32.store offset=4
                          local.get 0
                          local.get 2
                          i32.add
                          local.tee 7
                          local.get 1
                          i32.const 3
                          i32.shl
                          local.tee 1
                          local.get 2
                          i32.sub
                          local.tee 2
                          i32.const 1
                          i32.or
                          i32.store offset=4
                          local.get 0
                          local.get 1
                          i32.add
                          local.get 2
                          i32.store
                          i32.const 0
                          i32.load offset=1053648
                          local.tee 5
                          br_if 2 (;@9;)
                          br 3 (;@8;)
                        end
                        local.get 7
                        i32.const -8
                        i32.and
                        i32.const 1053376
                        i32.add
                        local.set 1
                        i32.const 0
                        i32.load offset=1053656
                        local.set 0
                        block  ;; label = @11
                          block  ;; label = @12
                            i32.const 0
                            i32.load offset=1053640
                            local.tee 8
                            i32.const 1
                            local.get 7
                            i32.const 3
                            i32.shr_u
                            i32.shl
                            local.tee 7
                            i32.and
                            i32.eqz
                            br_if 0 (;@12;)
                            local.get 1
                            i32.load offset=8
                            local.set 7
                            br 1 (;@11;)
                          end
                          i32.const 0
                          local.get 8
                          local.get 7
                          i32.or
                          i32.store offset=1053640
                          local.get 1
                          local.set 7
                        end
                        local.get 1
                        local.get 0
                        i32.store offset=8
                        local.get 7
                        local.get 0
                        i32.store offset=12
                        local.get 0
                        local.get 1
                        i32.store offset=12
                        local.get 0
                        local.get 7
                        i32.store offset=8
                        br 3 (;@7;)
                      end
                      local.get 6
                      local.get 5
                      local.get 2
                      i32.add
                      local.tee 0
                      i32.const 3
                      i32.or
                      i32.store offset=4
                      local.get 6
                      local.get 0
                      i32.add
                      local.tee 0
                      local.get 0
                      i32.load offset=4
                      i32.const 1
                      i32.or
                      i32.store offset=4
                      br 3 (;@6;)
                    end
                    local.get 5
                    i32.const -8
                    i32.and
                    i32.const 1053376
                    i32.add
                    local.set 1
                    i32.const 0
                    i32.load offset=1053656
                    local.set 0
                    block  ;; label = @9
                      block  ;; label = @10
                        i32.const 0
                        i32.load offset=1053640
                        local.tee 6
                        i32.const 1
                        local.get 5
                        i32.const 3
                        i32.shr_u
                        i32.shl
                        local.tee 5
                        i32.and
                        i32.eqz
                        br_if 0 (;@10;)
                        local.get 1
                        i32.load offset=8
                        local.set 5
                        br 1 (;@9;)
                      end
                      i32.const 0
                      local.get 6
                      local.get 5
                      i32.or
                      i32.store offset=1053640
                      local.get 1
                      local.set 5
                    end
                    local.get 1
                    local.get 0
                    i32.store offset=8
                    local.get 5
                    local.get 0
                    i32.store offset=12
                    local.get 0
                    local.get 1
                    i32.store offset=12
                    local.get 0
                    local.get 5
                    i32.store offset=8
                  end
                  i32.const 0
                  local.get 7
                  i32.store offset=1053656
                  i32.const 0
                  local.get 2
                  i32.store offset=1053648
                  local.get 8
                  return
                end
                i32.const 0
                local.get 2
                i32.store offset=1053656
                i32.const 0
                local.get 5
                i32.store offset=1053648
              end
              local.get 6
              i32.const 8
              i32.add
              return
            end
            block  ;; label = @5
              local.get 0
              local.get 6
              i32.or
              br_if 0 (;@5;)
              i32.const 0
              local.set 6
              local.get 3
              i32.const 2
              local.get 4
              i32.shl
              local.tee 0
              i32.const 0
              local.get 0
              i32.sub
              i32.or
              i32.and
              local.tee 0
              i32.eqz
              br_if 3 (;@2;)
              local.get 0
              i32.const 0
              local.get 0
              i32.sub
              i32.and
              i32.ctz
              i32.const 2
              i32.shl
              i32.const 1053232
              i32.add
              i32.load
              local.set 0
            end
            local.get 0
            i32.eqz
            br_if 1 (;@3;)
          end
          loop  ;; label = @4
            local.get 0
            i32.load offset=4
            i32.const -8
            i32.and
            local.tee 5
            local.get 2
            i32.ge_u
            local.get 5
            local.get 2
            i32.sub
            local.tee 8
            local.get 1
            i32.lt_u
            i32.and
            local.set 7
            block  ;; label = @5
              local.get 0
              i32.load offset=16
              local.tee 5
              br_if 0 (;@5;)
              local.get 0
              i32.const 20
              i32.add
              i32.load
              local.set 5
            end
            local.get 0
            local.get 6
            local.get 7
            select
            local.set 6
            local.get 8
            local.get 1
            local.get 7
            select
            local.set 1
            local.get 5
            local.set 0
            local.get 5
            br_if 0 (;@4;)
          end
        end
        local.get 6
        i32.eqz
        br_if 0 (;@2;)
        block  ;; label = @3
          i32.const 0
          i32.load offset=1053648
          local.tee 0
          local.get 2
          i32.lt_u
          br_if 0 (;@3;)
          local.get 1
          local.get 0
          local.get 2
          i32.sub
          i32.ge_u
          br_if 1 (;@2;)
        end
        local.get 6
        call 27
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.const 16
            i32.lt_u
            br_if 0 (;@4;)
            local.get 6
            local.get 2
            i32.const 3
            i32.or
            i32.store offset=4
            local.get 6
            local.get 2
            i32.add
            local.tee 0
            local.get 1
            i32.const 1
            i32.or
            i32.store offset=4
            local.get 0
            local.get 1
            i32.add
            local.get 1
            i32.store
            block  ;; label = @5
              local.get 1
              i32.const 256
              i32.lt_u
              br_if 0 (;@5;)
              local.get 0
              local.get 1
              call 68
              br 2 (;@3;)
            end
            local.get 1
            i32.const -8
            i32.and
            i32.const 1053376
            i32.add
            local.set 2
            block  ;; label = @5
              block  ;; label = @6
                i32.const 0
                i32.load offset=1053640
                local.tee 5
                i32.const 1
                local.get 1
                i32.const 3
                i32.shr_u
                i32.shl
                local.tee 1
                i32.and
                i32.eqz
                br_if 0 (;@6;)
                local.get 2
                i32.load offset=8
                local.set 1
                br 1 (;@5;)
              end
              i32.const 0
              local.get 5
              local.get 1
              i32.or
              i32.store offset=1053640
              local.get 2
              local.set 1
            end
            local.get 2
            local.get 0
            i32.store offset=8
            local.get 1
            local.get 0
            i32.store offset=12
            local.get 0
            local.get 2
            i32.store offset=12
            local.get 0
            local.get 1
            i32.store offset=8
            br 1 (;@3;)
          end
          local.get 6
          local.get 1
          local.get 2
          i32.add
          local.tee 0
          i32.const 3
          i32.or
          i32.store offset=4
          local.get 6
          local.get 0
          i32.add
          local.tee 0
          local.get 0
          i32.load offset=4
          i32.const 1
          i32.or
          i32.store offset=4
        end
        local.get 6
        i32.const 8
        i32.add
        return
      end
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        block  ;; label = @11
                          i32.const 0
                          i32.load offset=1053648
                          local.tee 0
                          local.get 2
                          i32.ge_u
                          br_if 0 (;@11;)
                          block  ;; label = @12
                            i32.const 0
                            i32.load offset=1053652
                            local.tee 0
                            local.get 2
                            i32.gt_u
                            br_if 0 (;@12;)
                            i32.const 0
                            local.set 1
                            local.get 2
                            i32.const 65583
                            i32.add
                            local.tee 5
                            i32.const 16
                            i32.shr_u
                            memory.grow
                            local.tee 0
                            i32.const -1
                            i32.eq
                            local.tee 6
                            br_if 11 (;@1;)
                            local.get 0
                            i32.const 16
                            i32.shl
                            local.tee 7
                            i32.eqz
                            br_if 11 (;@1;)
                            i32.const 0
                            i32.const 0
                            i32.load offset=1053664
                            i32.const 0
                            local.get 5
                            i32.const -65536
                            i32.and
                            local.get 6
                            select
                            local.tee 8
                            i32.add
                            local.tee 0
                            i32.store offset=1053664
                            i32.const 0
                            i32.const 0
                            i32.load offset=1053668
                            local.tee 1
                            local.get 0
                            local.get 1
                            local.get 0
                            i32.gt_u
                            select
                            i32.store offset=1053668
                            block  ;; label = @13
                              block  ;; label = @14
                                block  ;; label = @15
                                  i32.const 0
                                  i32.load offset=1053660
                                  local.tee 1
                                  i32.eqz
                                  br_if 0 (;@15;)
                                  i32.const 1053360
                                  local.set 0
                                  loop  ;; label = @16
                                    local.get 0
                                    i32.load
                                    local.tee 5
                                    local.get 0
                                    i32.load offset=4
                                    local.tee 6
                                    i32.add
                                    local.get 7
                                    i32.eq
                                    br_if 2 (;@14;)
                                    local.get 0
                                    i32.load offset=8
                                    local.tee 0
                                    br_if 0 (;@16;)
                                    br 3 (;@13;)
                                  end
                                end
                                i32.const 0
                                i32.load offset=1053676
                                local.tee 0
                                i32.eqz
                                br_if 4 (;@10;)
                                local.get 0
                                local.get 7
                                i32.gt_u
                                br_if 4 (;@10;)
                                br 11 (;@3;)
                              end
                              local.get 0
                              i32.load offset=12
                              br_if 0 (;@13;)
                              local.get 5
                              local.get 1
                              i32.gt_u
                              br_if 0 (;@13;)
                              local.get 1
                              local.get 7
                              i32.lt_u
                              br_if 4 (;@9;)
                            end
                            i32.const 0
                            i32.const 0
                            i32.load offset=1053676
                            local.tee 0
                            local.get 7
                            local.get 0
                            local.get 7
                            i32.lt_u
                            select
                            i32.store offset=1053676
                            local.get 7
                            local.get 8
                            i32.add
                            local.set 5
                            i32.const 1053360
                            local.set 0
                            block  ;; label = @13
                              block  ;; label = @14
                                block  ;; label = @15
                                  loop  ;; label = @16
                                    local.get 0
                                    i32.load
                                    local.get 5
                                    i32.eq
                                    br_if 1 (;@15;)
                                    local.get 0
                                    i32.load offset=8
                                    local.tee 0
                                    br_if 0 (;@16;)
                                    br 2 (;@14;)
                                  end
                                end
                                local.get 0
                                i32.load offset=12
                                i32.eqz
                                br_if 1 (;@13;)
                              end
                              i32.const 1053360
                              local.set 0
                              block  ;; label = @14
                                loop  ;; label = @15
                                  block  ;; label = @16
                                    local.get 0
                                    i32.load
                                    local.tee 5
                                    local.get 1
                                    i32.gt_u
                                    br_if 0 (;@16;)
                                    local.get 5
                                    local.get 0
                                    i32.load offset=4
                                    i32.add
                                    local.tee 5
                                    local.get 1
                                    i32.gt_u
                                    br_if 2 (;@14;)
                                  end
                                  local.get 0
                                  i32.load offset=8
                                  local.set 0
                                  br 0 (;@15;)
                                end
                              end
                              i32.const 0
                              local.get 7
                              i32.store offset=1053660
                              i32.const 0
                              local.get 8
                              i32.const -40
                              i32.add
                              local.tee 0
                              i32.store offset=1053652
                              local.get 7
                              local.get 0
                              i32.const 1
                              i32.or
                              i32.store offset=4
                              local.get 7
                              local.get 0
                              i32.add
                              i32.const 40
                              i32.store offset=4
                              i32.const 0
                              i32.const 2097152
                              i32.store offset=1053672
                              local.get 1
                              local.get 5
                              i32.const -32
                              i32.add
                              i32.const -8
                              i32.and
                              i32.const -8
                              i32.add
                              local.tee 0
                              local.get 0
                              local.get 1
                              i32.const 16
                              i32.add
                              i32.lt_u
                              select
                              local.tee 6
                              i32.const 27
                              i32.store offset=4
                              i32.const 0
                              i64.load offset=1053360 align=4
                              local.set 9
                              local.get 6
                              i32.const 16
                              i32.add
                              i32.const 0
                              i64.load offset=1053368 align=4
                              i64.store align=4
                              local.get 6
                              local.get 9
                              i64.store offset=8 align=4
                              i32.const 0
                              local.get 8
                              i32.store offset=1053364
                              i32.const 0
                              local.get 7
                              i32.store offset=1053360
                              i32.const 0
                              local.get 6
                              i32.const 8
                              i32.add
                              i32.store offset=1053368
                              i32.const 0
                              i32.const 0
                              i32.store offset=1053372
                              local.get 6
                              i32.const 28
                              i32.add
                              local.set 0
                              loop  ;; label = @14
                                local.get 0
                                i32.const 7
                                i32.store
                                local.get 0
                                i32.const 4
                                i32.add
                                local.tee 0
                                local.get 5
                                i32.lt_u
                                br_if 0 (;@14;)
                              end
                              local.get 6
                              local.get 1
                              i32.eq
                              br_if 11 (;@2;)
                              local.get 6
                              local.get 6
                              i32.load offset=4
                              i32.const -2
                              i32.and
                              i32.store offset=4
                              local.get 1
                              local.get 6
                              local.get 1
                              i32.sub
                              local.tee 0
                              i32.const 1
                              i32.or
                              i32.store offset=4
                              local.get 6
                              local.get 0
                              i32.store
                              block  ;; label = @14
                                local.get 0
                                i32.const 256
                                i32.lt_u
                                br_if 0 (;@14;)
                                local.get 1
                                local.get 0
                                call 68
                                br 12 (;@2;)
                              end
                              local.get 0
                              i32.const -8
                              i32.and
                              i32.const 1053376
                              i32.add
                              local.set 5
                              block  ;; label = @14
                                block  ;; label = @15
                                  i32.const 0
                                  i32.load offset=1053640
                                  local.tee 7
                                  i32.const 1
                                  local.get 0
                                  i32.const 3
                                  i32.shr_u
                                  i32.shl
                                  local.tee 0
                                  i32.and
                                  i32.eqz
                                  br_if 0 (;@15;)
                                  local.get 5
                                  i32.load offset=8
                                  local.set 0
                                  br 1 (;@14;)
                                end
                                i32.const 0
                                local.get 7
                                local.get 0
                                i32.or
                                i32.store offset=1053640
                                local.get 5
                                local.set 0
                              end
                              local.get 5
                              local.get 1
                              i32.store offset=8
                              local.get 0
                              local.get 1
                              i32.store offset=12
                              local.get 1
                              local.get 5
                              i32.store offset=12
                              local.get 1
                              local.get 0
                              i32.store offset=8
                              br 11 (;@2;)
                            end
                            local.get 0
                            local.get 7
                            i32.store
                            local.get 0
                            local.get 0
                            i32.load offset=4
                            local.get 8
                            i32.add
                            i32.store offset=4
                            local.get 7
                            local.get 2
                            i32.const 3
                            i32.or
                            i32.store offset=4
                            local.get 5
                            local.get 7
                            local.get 2
                            i32.add
                            local.tee 0
                            i32.sub
                            local.set 2
                            block  ;; label = @13
                              local.get 5
                              i32.const 0
                              i32.load offset=1053660
                              i32.eq
                              br_if 0 (;@13;)
                              local.get 5
                              i32.const 0
                              i32.load offset=1053656
                              i32.eq
                              br_if 5 (;@8;)
                              local.get 5
                              i32.load offset=4
                              local.tee 1
                              i32.const 3
                              i32.and
                              i32.const 1
                              i32.ne
                              br_if 8 (;@5;)
                              block  ;; label = @14
                                block  ;; label = @15
                                  local.get 1
                                  i32.const -8
                                  i32.and
                                  local.tee 6
                                  i32.const 256
                                  i32.lt_u
                                  br_if 0 (;@15;)
                                  local.get 5
                                  call 27
                                  br 1 (;@14;)
                                end
                                block  ;; label = @15
                                  local.get 5
                                  i32.const 12
                                  i32.add
                                  i32.load
                                  local.tee 8
                                  local.get 5
                                  i32.const 8
                                  i32.add
                                  i32.load
                                  local.tee 4
                                  i32.eq
                                  br_if 0 (;@15;)
                                  local.get 4
                                  local.get 8
                                  i32.store offset=12
                                  local.get 8
                                  local.get 4
                                  i32.store offset=8
                                  br 1 (;@14;)
                                end
                                i32.const 0
                                i32.const 0
                                i32.load offset=1053640
                                i32.const -2
                                local.get 1
                                i32.const 3
                                i32.shr_u
                                i32.rotl
                                i32.and
                                i32.store offset=1053640
                              end
                              local.get 6
                              local.get 2
                              i32.add
                              local.set 2
                              local.get 5
                              local.get 6
                              i32.add
                              local.tee 5
                              i32.load offset=4
                              local.set 1
                              br 8 (;@5;)
                            end
                            i32.const 0
                            local.get 0
                            i32.store offset=1053660
                            i32.const 0
                            i32.const 0
                            i32.load offset=1053652
                            local.get 2
                            i32.add
                            local.tee 2
                            i32.store offset=1053652
                            local.get 0
                            local.get 2
                            i32.const 1
                            i32.or
                            i32.store offset=4
                            br 8 (;@4;)
                          end
                          i32.const 0
                          local.get 0
                          local.get 2
                          i32.sub
                          local.tee 1
                          i32.store offset=1053652
                          i32.const 0
                          i32.const 0
                          i32.load offset=1053660
                          local.tee 0
                          local.get 2
                          i32.add
                          local.tee 5
                          i32.store offset=1053660
                          local.get 5
                          local.get 1
                          i32.const 1
                          i32.or
                          i32.store offset=4
                          local.get 0
                          local.get 2
                          i32.const 3
                          i32.or
                          i32.store offset=4
                          local.get 0
                          i32.const 8
                          i32.add
                          local.set 1
                          br 10 (;@1;)
                        end
                        i32.const 0
                        i32.load offset=1053656
                        local.set 1
                        local.get 0
                        local.get 2
                        i32.sub
                        local.tee 5
                        i32.const 16
                        i32.lt_u
                        br_if 3 (;@7;)
                        i32.const 0
                        local.get 5
                        i32.store offset=1053648
                        i32.const 0
                        local.get 1
                        local.get 2
                        i32.add
                        local.tee 7
                        i32.store offset=1053656
                        local.get 7
                        local.get 5
                        i32.const 1
                        i32.or
                        i32.store offset=4
                        local.get 1
                        local.get 0
                        i32.add
                        local.get 5
                        i32.store
                        local.get 1
                        local.get 2
                        i32.const 3
                        i32.or
                        i32.store offset=4
                        br 4 (;@6;)
                      end
                      i32.const 0
                      local.get 7
                      i32.store offset=1053676
                      br 6 (;@3;)
                    end
                    local.get 0
                    local.get 6
                    local.get 8
                    i32.add
                    i32.store offset=4
                    i32.const 0
                    i32.const 0
                    i32.load offset=1053660
                    local.tee 0
                    i32.const 15
                    i32.add
                    i32.const -8
                    i32.and
                    local.tee 1
                    i32.const -8
                    i32.add
                    i32.store offset=1053660
                    i32.const 0
                    local.get 0
                    local.get 1
                    i32.sub
                    i32.const 0
                    i32.load offset=1053652
                    local.get 8
                    i32.add
                    local.tee 5
                    i32.add
                    i32.const 8
                    i32.add
                    local.tee 7
                    i32.store offset=1053652
                    local.get 1
                    i32.const -4
                    i32.add
                    local.get 7
                    i32.const 1
                    i32.or
                    i32.store
                    local.get 0
                    local.get 5
                    i32.add
                    i32.const 40
                    i32.store offset=4
                    i32.const 0
                    i32.const 2097152
                    i32.store offset=1053672
                    br 6 (;@2;)
                  end
                  i32.const 0
                  local.get 0
                  i32.store offset=1053656
                  i32.const 0
                  i32.const 0
                  i32.load offset=1053648
                  local.get 2
                  i32.add
                  local.tee 2
                  i32.store offset=1053648
                  local.get 0
                  local.get 2
                  i32.const 1
                  i32.or
                  i32.store offset=4
                  local.get 0
                  local.get 2
                  i32.add
                  local.get 2
                  i32.store
                  br 3 (;@4;)
                end
                i32.const 0
                i32.const 0
                i32.store offset=1053656
                i32.const 0
                i32.const 0
                i32.store offset=1053648
                local.get 1
                local.get 0
                i32.const 3
                i32.or
                i32.store offset=4
                local.get 1
                local.get 0
                i32.add
                local.tee 0
                local.get 0
                i32.load offset=4
                i32.const 1
                i32.or
                i32.store offset=4
              end
              local.get 1
              i32.const 8
              i32.add
              return
            end
            local.get 5
            local.get 1
            i32.const -2
            i32.and
            i32.store offset=4
            local.get 0
            local.get 2
            i32.const 1
            i32.or
            i32.store offset=4
            local.get 0
            local.get 2
            i32.add
            local.get 2
            i32.store
            block  ;; label = @5
              local.get 2
              i32.const 256
              i32.lt_u
              br_if 0 (;@5;)
              local.get 0
              local.get 2
              call 68
              br 1 (;@4;)
            end
            local.get 2
            i32.const -8
            i32.and
            i32.const 1053376
            i32.add
            local.set 1
            block  ;; label = @5
              block  ;; label = @6
                i32.const 0
                i32.load offset=1053640
                local.tee 5
                i32.const 1
                local.get 2
                i32.const 3
                i32.shr_u
                i32.shl
                local.tee 2
                i32.and
                i32.eqz
                br_if 0 (;@6;)
                local.get 1
                i32.load offset=8
                local.set 2
                br 1 (;@5;)
              end
              i32.const 0
              local.get 5
              local.get 2
              i32.or
              i32.store offset=1053640
              local.get 1
              local.set 2
            end
            local.get 1
            local.get 0
            i32.store offset=8
            local.get 2
            local.get 0
            i32.store offset=12
            local.get 0
            local.get 1
            i32.store offset=12
            local.get 0
            local.get 2
            i32.store offset=8
          end
          local.get 7
          i32.const 8
          i32.add
          return
        end
        i32.const 0
        i32.const 4095
        i32.store offset=1053680
        i32.const 0
        local.get 8
        i32.store offset=1053364
        i32.const 0
        local.get 7
        i32.store offset=1053360
        i32.const 0
        i32.const 1053376
        i32.store offset=1053388
        i32.const 0
        i32.const 1053384
        i32.store offset=1053396
        i32.const 0
        i32.const 1053376
        i32.store offset=1053384
        i32.const 0
        i32.const 1053392
        i32.store offset=1053404
        i32.const 0
        i32.const 1053384
        i32.store offset=1053392
        i32.const 0
        i32.const 1053400
        i32.store offset=1053412
        i32.const 0
        i32.const 1053392
        i32.store offset=1053400
        i32.const 0
        i32.const 1053408
        i32.store offset=1053420
        i32.const 0
        i32.const 1053400
        i32.store offset=1053408
        i32.const 0
        i32.const 1053416
        i32.store offset=1053428
        i32.const 0
        i32.const 1053408
        i32.store offset=1053416
        i32.const 0
        i32.const 1053424
        i32.store offset=1053436
        i32.const 0
        i32.const 1053416
        i32.store offset=1053424
        i32.const 0
        i32.const 1053432
        i32.store offset=1053444
        i32.const 0
        i32.const 1053424
        i32.store offset=1053432
        i32.const 0
        i32.const 0
        i32.store offset=1053372
        i32.const 0
        i32.const 1053440
        i32.store offset=1053452
        i32.const 0
        i32.const 1053432
        i32.store offset=1053440
        i32.const 0
        i32.const 1053440
        i32.store offset=1053448
        i32.const 0
        i32.const 1053448
        i32.store offset=1053460
        i32.const 0
        i32.const 1053448
        i32.store offset=1053456
        i32.const 0
        i32.const 1053456
        i32.store offset=1053468
        i32.const 0
        i32.const 1053456
        i32.store offset=1053464
        i32.const 0
        i32.const 1053464
        i32.store offset=1053476
        i32.const 0
        i32.const 1053464
        i32.store offset=1053472
        i32.const 0
        i32.const 1053472
        i32.store offset=1053484
        i32.const 0
        i32.const 1053472
        i32.store offset=1053480
        i32.const 0
        i32.const 1053480
        i32.store offset=1053492
        i32.const 0
        i32.const 1053480
        i32.store offset=1053488
        i32.const 0
        i32.const 1053488
        i32.store offset=1053500
        i32.const 0
        i32.const 1053488
        i32.store offset=1053496
        i32.const 0
        i32.const 1053496
        i32.store offset=1053508
        i32.const 0
        i32.const 1053496
        i32.store offset=1053504
        i32.const 0
        i32.const 1053504
        i32.store offset=1053516
        i32.const 0
        i32.const 1053512
        i32.store offset=1053524
        i32.const 0
        i32.const 1053504
        i32.store offset=1053512
        i32.const 0
        i32.const 1053520
        i32.store offset=1053532
        i32.const 0
        i32.const 1053512
        i32.store offset=1053520
        i32.const 0
        i32.const 1053528
        i32.store offset=1053540
        i32.const 0
        i32.const 1053520
        i32.store offset=1053528
        i32.const 0
        i32.const 1053536
        i32.store offset=1053548
        i32.const 0
        i32.const 1053528
        i32.store offset=1053536
        i32.const 0
        i32.const 1053544
        i32.store offset=1053556
        i32.const 0
        i32.const 1053536
        i32.store offset=1053544
        i32.const 0
        i32.const 1053552
        i32.store offset=1053564
        i32.const 0
        i32.const 1053544
        i32.store offset=1053552
        i32.const 0
        i32.const 1053560
        i32.store offset=1053572
        i32.const 0
        i32.const 1053552
        i32.store offset=1053560
        i32.const 0
        i32.const 1053568
        i32.store offset=1053580
        i32.const 0
        i32.const 1053560
        i32.store offset=1053568
        i32.const 0
        i32.const 1053576
        i32.store offset=1053588
        i32.const 0
        i32.const 1053568
        i32.store offset=1053576
        i32.const 0
        i32.const 1053584
        i32.store offset=1053596
        i32.const 0
        i32.const 1053576
        i32.store offset=1053584
        i32.const 0
        i32.const 1053592
        i32.store offset=1053604
        i32.const 0
        i32.const 1053584
        i32.store offset=1053592
        i32.const 0
        i32.const 1053600
        i32.store offset=1053612
        i32.const 0
        i32.const 1053592
        i32.store offset=1053600
        i32.const 0
        i32.const 1053608
        i32.store offset=1053620
        i32.const 0
        i32.const 1053600
        i32.store offset=1053608
        i32.const 0
        i32.const 1053616
        i32.store offset=1053628
        i32.const 0
        i32.const 1053608
        i32.store offset=1053616
        i32.const 0
        i32.const 1053624
        i32.store offset=1053636
        i32.const 0
        i32.const 1053616
        i32.store offset=1053624
        i32.const 0
        local.get 7
        i32.store offset=1053660
        i32.const 0
        i32.const 1053624
        i32.store offset=1053632
        i32.const 0
        local.get 8
        i32.const -40
        i32.add
        local.tee 0
        i32.store offset=1053652
        local.get 7
        local.get 0
        i32.const 1
        i32.or
        i32.store offset=4
        local.get 7
        local.get 0
        i32.add
        i32.const 40
        i32.store offset=4
        i32.const 0
        i32.const 2097152
        i32.store offset=1053672
      end
      i32.const 0
      local.set 1
      i32.const 0
      i32.load offset=1053652
      local.tee 0
      local.get 2
      i32.le_u
      br_if 0 (;@1;)
      i32.const 0
      local.get 0
      local.get 2
      i32.sub
      local.tee 1
      i32.store offset=1053652
      i32.const 0
      i32.const 0
      i32.load offset=1053660
      local.tee 0
      local.get 2
      i32.add
      local.tee 5
      i32.store offset=1053660
      local.get 5
      local.get 1
      i32.const 1
      i32.or
      i32.store offset=4
      local.get 0
      local.get 2
      i32.const 3
      i32.or
      i32.store offset=4
      local.get 0
      i32.const 8
      i32.add
      return
    end
    local.get 1)
  (func (;5;) (type 0) (param i32)
    (local i32 i32 i32)
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 0
            i32.load
            br_table 0 (;@4;) 1 (;@3;) 3 (;@1;)
          end
          local.get 0
          i32.const 8
          i32.add
          i32.load
          i32.eqz
          br_if 2 (;@1;)
          local.get 0
          i32.load offset=4
          local.set 1
          br 1 (;@2;)
        end
        local.get 0
        i32.load8_u offset=4
        i32.const 3
        i32.ne
        br_if 1 (;@1;)
        local.get 0
        i32.const 8
        i32.add
        i32.load
        local.tee 1
        i32.load
        local.tee 2
        local.get 1
        i32.load offset=4
        local.tee 3
        i32.load
        call_indirect (type 0)
        local.get 3
        i32.load offset=4
        i32.eqz
        br_if 0 (;@2;)
        local.get 2
        call 6
      end
      local.get 1
      call 6
    end
    local.get 0
    call 6)
  (func (;6;) (type 0) (param i32)
    (local i32 i32 i32 i32 i32)
    local.get 0
    i32.const -8
    i32.add
    local.tee 1
    local.get 0
    i32.const -4
    i32.add
    i32.load
    local.tee 2
    i32.const -8
    i32.and
    local.tee 0
    i32.add
    local.set 3
    block  ;; label = @1
      block  ;; label = @2
        local.get 2
        i32.const 1
        i32.and
        br_if 0 (;@2;)
        local.get 2
        i32.const 3
        i32.and
        i32.eqz
        br_if 1 (;@1;)
        local.get 1
        i32.load
        local.tee 2
        local.get 0
        i32.add
        local.set 0
        block  ;; label = @3
          local.get 1
          local.get 2
          i32.sub
          local.tee 1
          i32.const 0
          i32.load offset=1053656
          i32.ne
          br_if 0 (;@3;)
          local.get 3
          i32.load offset=4
          i32.const 3
          i32.and
          i32.const 3
          i32.ne
          br_if 1 (;@2;)
          i32.const 0
          local.get 0
          i32.store offset=1053648
          local.get 3
          local.get 3
          i32.load offset=4
          i32.const -2
          i32.and
          i32.store offset=4
          local.get 1
          local.get 0
          i32.const 1
          i32.or
          i32.store offset=4
          local.get 1
          local.get 0
          i32.add
          local.get 0
          i32.store
          return
        end
        block  ;; label = @3
          local.get 2
          i32.const 256
          i32.lt_u
          br_if 0 (;@3;)
          local.get 1
          call 27
          br 1 (;@2;)
        end
        block  ;; label = @3
          local.get 1
          i32.const 12
          i32.add
          i32.load
          local.tee 4
          local.get 1
          i32.const 8
          i32.add
          i32.load
          local.tee 5
          i32.eq
          br_if 0 (;@3;)
          local.get 5
          local.get 4
          i32.store offset=12
          local.get 4
          local.get 5
          i32.store offset=8
          br 1 (;@2;)
        end
        i32.const 0
        i32.const 0
        i32.load offset=1053640
        i32.const -2
        local.get 2
        i32.const 3
        i32.shr_u
        i32.rotl
        i32.and
        i32.store offset=1053640
      end
      block  ;; label = @2
        block  ;; label = @3
          local.get 3
          i32.load offset=4
          local.tee 2
          i32.const 2
          i32.and
          i32.eqz
          br_if 0 (;@3;)
          local.get 3
          local.get 2
          i32.const -2
          i32.and
          i32.store offset=4
          local.get 1
          local.get 0
          i32.const 1
          i32.or
          i32.store offset=4
          local.get 1
          local.get 0
          i32.add
          local.get 0
          i32.store
          br 1 (;@2;)
        end
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                local.get 3
                i32.const 0
                i32.load offset=1053660
                i32.eq
                br_if 0 (;@6;)
                local.get 3
                i32.const 0
                i32.load offset=1053656
                i32.eq
                br_if 1 (;@5;)
                local.get 2
                i32.const -8
                i32.and
                local.tee 4
                local.get 0
                i32.add
                local.set 0
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 4
                    i32.const 256
                    i32.lt_u
                    br_if 0 (;@8;)
                    local.get 3
                    call 27
                    br 1 (;@7;)
                  end
                  block  ;; label = @8
                    local.get 3
                    i32.const 12
                    i32.add
                    i32.load
                    local.tee 4
                    local.get 3
                    i32.const 8
                    i32.add
                    i32.load
                    local.tee 3
                    i32.eq
                    br_if 0 (;@8;)
                    local.get 3
                    local.get 4
                    i32.store offset=12
                    local.get 4
                    local.get 3
                    i32.store offset=8
                    br 1 (;@7;)
                  end
                  i32.const 0
                  i32.const 0
                  i32.load offset=1053640
                  i32.const -2
                  local.get 2
                  i32.const 3
                  i32.shr_u
                  i32.rotl
                  i32.and
                  i32.store offset=1053640
                end
                local.get 1
                local.get 0
                i32.const 1
                i32.or
                i32.store offset=4
                local.get 1
                local.get 0
                i32.add
                local.get 0
                i32.store
                local.get 1
                i32.const 0
                i32.load offset=1053656
                i32.ne
                br_if 4 (;@2;)
                i32.const 0
                local.get 0
                i32.store offset=1053648
                return
              end
              i32.const 0
              local.get 1
              i32.store offset=1053660
              i32.const 0
              i32.const 0
              i32.load offset=1053652
              local.get 0
              i32.add
              local.tee 0
              i32.store offset=1053652
              local.get 1
              local.get 0
              i32.const 1
              i32.or
              i32.store offset=4
              local.get 1
              i32.const 0
              i32.load offset=1053656
              i32.eq
              br_if 1 (;@4;)
              br 2 (;@3;)
            end
            i32.const 0
            local.get 1
            i32.store offset=1053656
            i32.const 0
            i32.const 0
            i32.load offset=1053648
            local.get 0
            i32.add
            local.tee 0
            i32.store offset=1053648
            local.get 1
            local.get 0
            i32.const 1
            i32.or
            i32.store offset=4
            local.get 1
            local.get 0
            i32.add
            local.get 0
            i32.store
            return
          end
          i32.const 0
          i32.const 0
          i32.store offset=1053648
          i32.const 0
          i32.const 0
          i32.store offset=1053656
        end
        local.get 0
        i32.const 0
        i32.load offset=1053672
        local.tee 4
        i32.le_u
        br_if 1 (;@1;)
        i32.const 0
        i32.load offset=1053660
        local.tee 3
        i32.eqz
        br_if 1 (;@1;)
        i32.const 0
        local.set 1
        block  ;; label = @3
          i32.const 0
          i32.load offset=1053652
          local.tee 5
          i32.const 41
          i32.lt_u
          br_if 0 (;@3;)
          i32.const 1053360
          local.set 0
          loop  ;; label = @4
            block  ;; label = @5
              local.get 0
              i32.load
              local.tee 2
              local.get 3
              i32.gt_u
              br_if 0 (;@5;)
              local.get 2
              local.get 0
              i32.load offset=4
              i32.add
              local.get 3
              i32.gt_u
              br_if 2 (;@3;)
            end
            local.get 0
            i32.load offset=8
            local.tee 0
            br_if 0 (;@4;)
          end
        end
        block  ;; label = @3
          i32.const 0
          i32.load offset=1053368
          local.tee 0
          i32.eqz
          br_if 0 (;@3;)
          i32.const 0
          local.set 1
          loop  ;; label = @4
            local.get 1
            i32.const 1
            i32.add
            local.set 1
            local.get 0
            i32.load offset=8
            local.tee 0
            br_if 0 (;@4;)
          end
        end
        i32.const 0
        local.get 1
        i32.const 4095
        local.get 1
        i32.const 4095
        i32.gt_u
        select
        i32.store offset=1053680
        local.get 5
        local.get 4
        i32.le_u
        br_if 1 (;@1;)
        i32.const 0
        i32.const -1
        i32.store offset=1053672
        return
      end
      block  ;; label = @2
        local.get 0
        i32.const 256
        i32.lt_u
        br_if 0 (;@2;)
        local.get 1
        local.get 0
        call 68
        i32.const 0
        local.set 1
        i32.const 0
        i32.const 0
        i32.load offset=1053680
        i32.const -1
        i32.add
        local.tee 0
        i32.store offset=1053680
        local.get 0
        br_if 1 (;@1;)
        block  ;; label = @3
          i32.const 0
          i32.load offset=1053368
          local.tee 0
          i32.eqz
          br_if 0 (;@3;)
          i32.const 0
          local.set 1
          loop  ;; label = @4
            local.get 1
            i32.const 1
            i32.add
            local.set 1
            local.get 0
            i32.load offset=8
            local.tee 0
            br_if 0 (;@4;)
          end
        end
        i32.const 0
        local.get 1
        i32.const 4095
        local.get 1
        i32.const 4095
        i32.gt_u
        select
        i32.store offset=1053680
        return
      end
      local.get 0
      i32.const -8
      i32.and
      i32.const 1053376
      i32.add
      local.set 3
      block  ;; label = @2
        block  ;; label = @3
          i32.const 0
          i32.load offset=1053640
          local.tee 2
          i32.const 1
          local.get 0
          i32.const 3
          i32.shr_u
          i32.shl
          local.tee 0
          i32.and
          i32.eqz
          br_if 0 (;@3;)
          local.get 3
          i32.load offset=8
          local.set 0
          br 1 (;@2;)
        end
        i32.const 0
        local.get 2
        local.get 0
        i32.or
        i32.store offset=1053640
        local.get 3
        local.set 0
      end
      local.get 3
      local.get 1
      i32.store offset=8
      local.get 0
      local.get 1
      i32.store offset=12
      local.get 1
      local.get 3
      i32.store offset=12
      local.get 1
      local.get 0
      i32.store offset=8
    end)
  (func (;7;) (type 8) (param i32 i32 i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 64
    i32.sub
    local.tee 5
    global.set 0
    local.get 5
    local.get 1
    i32.store offset=12
    local.get 5
    local.get 0
    i32.store offset=8
    local.get 5
    local.get 3
    i32.store offset=20
    local.get 5
    local.get 2
    i32.store offset=16
    local.get 5
    i32.const 24
    i32.add
    i32.const 12
    i32.add
    i64.const 2
    i64.store align=4
    local.get 5
    i32.const 48
    i32.add
    i32.const 12
    i32.add
    i32.const 1
    i32.store
    local.get 5
    i32.const 2
    i32.store offset=28
    local.get 5
    i32.const 1049656
    i32.store offset=24
    local.get 5
    i32.const 2
    i32.store offset=52
    local.get 5
    local.get 5
    i32.const 48
    i32.add
    i32.store offset=32
    local.get 5
    local.get 5
    i32.const 16
    i32.add
    i32.store offset=56
    local.get 5
    local.get 5
    i32.const 8
    i32.add
    i32.store offset=48
    local.get 5
    i32.const 24
    i32.add
    local.get 4
    call 24
    unreachable)
  (func (;8;) (type 9) (param i32 i64 i32 i64)
    (local i32 i32 i32 i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 4
    global.set 0
    local.get 4
    i32.const 0
    i32.store offset=16
    local.get 4
    i64.const 1
    i64.store offset=8
    local.get 4
    local.get 3
    i64.store offset=24
    local.get 4
    i32.const 8
    i32.add
    local.get 4
    i32.const 24
    i32.add
    i32.const 8
    call 9
    local.get 4
    i32.load offset=12
    local.set 5
    local.get 4
    i32.load offset=16
    local.set 6
    block  ;; label = @1
      block  ;; label = @2
        local.get 4
        i32.load offset=8
        local.tee 7
        i32.eqz
        br_if 0 (;@2;)
        local.get 0
        i32.const 4
        i32.const 7
        local.get 1
        local.get 2
        i32.load
        local.get 2
        i32.load offset=8
        local.get 7
        local.get 6
        call 2
        select
        i32.store8
        local.get 5
        i32.eqz
        br_if 1 (;@1;)
        local.get 7
        call 6
        br 1 (;@1;)
      end
      local.get 5
      call 5
      local.get 0
      local.get 6
      i32.store offset=12
      local.get 0
      local.get 5
      i32.store offset=8
      local.get 0
      i32.const 0
      i32.store offset=4
      local.get 0
      i32.const 6
      i32.store8
    end
    block  ;; label = @1
      local.get 2
      i32.load offset=4
      i32.eqz
      br_if 0 (;@1;)
      local.get 2
      i32.load
      call 6
    end
    local.get 4
    i32.const 32
    i32.add
    global.set 0)
  (func (;9;) (type 10) (param i32 i32 i32)
    (local i32)
    block  ;; label = @1
      local.get 0
      i32.load offset=4
      local.get 0
      i32.load offset=8
      local.tee 3
      i32.sub
      local.get 2
      i32.ge_u
      br_if 0 (;@1;)
      local.get 0
      local.get 3
      local.get 2
      call 18
      local.get 0
      i32.load offset=8
      local.set 3
    end
    local.get 0
    i32.load
    local.get 3
    i32.add
    local.get 1
    local.get 2
    call 81
    drop
    local.get 0
    local.get 3
    local.get 2
    i32.add
    i32.store offset=8)
  (func (;10;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    local.get 0
    i32.load
    i32.store offset=12
    local.get 1
    i32.const 1053170
    i32.const 7
    local.get 2
    i32.const 12
    i32.add
    i32.const 1053180
    call 11
    local.set 0
    local.get 2
    i32.const 16
    i32.add
    global.set 0
    local.get 0)
  (func (;11;) (type 11) (param i32 i32 i32 i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 5
    global.set 0
    local.get 5
    local.get 0
    i32.load offset=20
    local.get 1
    local.get 2
    local.get 0
    i32.const 24
    i32.add
    i32.load
    i32.load offset=12
    call_indirect (type 1)
    i32.store8 offset=8
    local.get 5
    local.get 0
    i32.store offset=4
    local.get 5
    i32.const 0
    i32.store8 offset=9
    local.get 5
    i32.const 0
    i32.store
    local.get 5
    local.get 3
    local.get 4
    call 55
    local.set 0
    local.get 5
    i32.load8_u offset=8
    local.set 2
    block  ;; label = @1
      block  ;; label = @2
        local.get 0
        i32.load
        local.tee 1
        br_if 0 (;@2;)
        local.get 2
        i32.const 255
        i32.and
        i32.const 0
        i32.ne
        local.set 0
        br 1 (;@1;)
      end
      i32.const 1
      local.set 0
      local.get 2
      i32.const 255
      i32.and
      br_if 0 (;@1;)
      local.get 5
      i32.load offset=4
      local.set 2
      block  ;; label = @2
        local.get 1
        i32.const 1
        i32.ne
        br_if 0 (;@2;)
        local.get 5
        i32.load8_u offset=9
        i32.const 255
        i32.and
        i32.eqz
        br_if 0 (;@2;)
        local.get 2
        i32.load8_u offset=28
        i32.const 4
        i32.and
        br_if 0 (;@2;)
        i32.const 1
        local.set 0
        local.get 2
        i32.load offset=20
        i32.const 1049707
        i32.const 1
        local.get 2
        i32.const 24
        i32.add
        i32.load
        i32.load offset=12
        call_indirect (type 1)
        br_if 1 (;@1;)
      end
      local.get 2
      i32.load offset=20
      i32.const 1049396
      i32.const 1
      local.get 2
      i32.const 24
      i32.add
      i32.load
      i32.load offset=12
      call_indirect (type 1)
      local.set 0
    end
    local.get 5
    i32.const 16
    i32.add
    global.set 0
    local.get 0)
  (func (;12;) (type 0) (param i32))
  (func (;13;) (type 0) (param i32)
    block  ;; label = @1
      local.get 0
      i32.load8_u
      br_if 0 (;@1;)
      local.get 0
      i32.const 8
      i32.add
      i32.load
      i32.eqz
      br_if 0 (;@1;)
      local.get 0
      i32.load offset=4
      call 6
    end)
  (func (;14;) (type 10) (param i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    local.get 1
    i32.store offset=4
    local.get 3
    local.get 0
    i32.store
    local.get 3
    i32.const 8
    i32.add
    i32.const 16
    i32.add
    local.get 2
    i32.const 16
    i32.add
    i64.load align=4
    i64.store
    local.get 3
    i32.const 8
    i32.add
    i32.const 8
    i32.add
    local.get 2
    i32.const 8
    i32.add
    i64.load align=4
    i64.store
    local.get 3
    local.get 2
    i64.load align=4
    i64.store offset=8
    local.get 3
    local.get 3
    i32.const 4
    i32.add
    local.get 3
    i32.const 8
    i32.add
    call 15
    unreachable)
  (func (;15;) (type 10) (param i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 112
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    i32.const 1048744
    i32.store offset=12
    local.get 3
    local.get 0
    i32.store offset=8
    local.get 3
    i32.const 1048744
    i32.store offset=20
    local.get 3
    local.get 1
    i32.store offset=16
    local.get 3
    i32.const 2
    i32.store offset=28
    local.get 3
    i32.const 1049524
    i32.store offset=24
    block  ;; label = @1
      local.get 2
      i32.load
      br_if 0 (;@1;)
      local.get 3
      i32.const 76
      i32.add
      i32.const 1
      i32.store
      local.get 3
      i32.const 56
      i32.add
      i32.const 12
      i32.add
      i32.const 1
      i32.store
      local.get 3
      i32.const 88
      i32.add
      i32.const 12
      i32.add
      i64.const 3
      i64.store align=4
      local.get 3
      i32.const 4
      i32.store offset=92
      local.get 3
      i32.const 1049584
      i32.store offset=88
      local.get 3
      i32.const 2
      i32.store offset=60
      local.get 3
      local.get 3
      i32.const 56
      i32.add
      i32.store offset=96
      local.get 3
      local.get 3
      i32.const 16
      i32.add
      i32.store offset=72
      local.get 3
      local.get 3
      i32.const 8
      i32.add
      i32.store offset=64
      local.get 3
      local.get 3
      i32.const 24
      i32.add
      i32.store offset=56
      local.get 3
      i32.const 88
      i32.add
      i32.const 1049308
      call 24
      unreachable
    end
    local.get 3
    i32.const 32
    i32.add
    i32.const 16
    i32.add
    local.get 2
    i32.const 16
    i32.add
    i64.load align=4
    i64.store
    local.get 3
    i32.const 32
    i32.add
    i32.const 8
    i32.add
    local.get 2
    i32.const 8
    i32.add
    i64.load align=4
    i64.store
    local.get 3
    local.get 2
    i64.load align=4
    i64.store offset=32
    local.get 3
    i32.const 88
    i32.add
    i32.const 12
    i32.add
    i64.const 4
    i64.store align=4
    local.get 3
    i32.const 84
    i32.add
    i32.const 3
    i32.store
    local.get 3
    i32.const 76
    i32.add
    i32.const 1
    i32.store
    local.get 3
    i32.const 56
    i32.add
    i32.const 12
    i32.add
    i32.const 1
    i32.store
    local.get 3
    i32.const 4
    i32.store offset=92
    local.get 3
    i32.const 1049620
    i32.store offset=88
    local.get 3
    i32.const 2
    i32.store offset=60
    local.get 3
    local.get 3
    i32.const 56
    i32.add
    i32.store offset=96
    local.get 3
    local.get 3
    i32.const 32
    i32.add
    i32.store offset=80
    local.get 3
    local.get 3
    i32.const 16
    i32.add
    i32.store offset=72
    local.get 3
    local.get 3
    i32.const 8
    i32.add
    i32.store offset=64
    local.get 3
    local.get 3
    i32.const 24
    i32.add
    i32.store offset=56
    local.get 3
    i32.const 88
    i32.add
    i32.const 1049308
    call 24
    unreachable)
  (func (;16;) (type 12) (param i32 i32)
    (local i32 i32 i32 i32 i32 i32)
    global.get 0
    i32.const 64
    i32.sub
    local.tee 2
    global.set 0
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                local.get 1
                i32.load8_u
                br_table 0 (;@6;) 1 (;@5;) 2 (;@4;) 3 (;@3;) 0 (;@6;)
              end
              i32.const 0
              i32.load8_u offset=1053220
              drop
              i32.const 1
              call 4
              local.tee 1
              i32.eqz
              br_if 4 (;@1;)
              local.get 0
              i64.const 4294967297
              i64.store offset=4 align=4
              local.get 0
              local.get 1
              i32.store
              local.get 1
              i32.const 0
              i32.store8
              br 3 (;@2;)
            end
            i32.const 0
            i32.load8_u offset=1053220
            drop
            i32.const 1
            call 4
            local.tee 1
            i32.eqz
            br_if 3 (;@1;)
            local.get 0
            i64.const 4294967297
            i64.store offset=4 align=4
            local.get 0
            local.get 1
            i32.store
            local.get 1
            i32.const 1
            i32.store8
            br 2 (;@2;)
          end
          i32.const 0
          i32.load8_u offset=1053220
          drop
          i32.const 1
          call 4
          local.tee 1
          i32.eqz
          br_if 2 (;@1;)
          local.get 0
          i64.const 4294967297
          i64.store offset=4 align=4
          local.get 0
          local.get 1
          i32.store
          local.get 1
          i32.const 2
          i32.store8
          br 1 (;@2;)
        end
        local.get 2
        i32.const 33
        call 17
        i32.const 0
        local.set 3
        local.get 2
        i32.const 0
        i32.store offset=16
        local.get 2
        local.get 2
        i32.load offset=4
        local.tee 4
        i32.store offset=12
        local.get 2
        local.get 2
        i32.load
        local.tee 5
        i32.store offset=8
        local.get 1
        i32.const 1
        i32.add
        local.set 1
        block  ;; label = @3
          local.get 4
          i32.const 32
          i32.gt_u
          br_if 0 (;@3;)
          local.get 2
          i32.const 8
          i32.add
          i32.const 0
          i32.const 33
          call 18
          local.get 2
          i32.load offset=16
          local.set 3
          local.get 2
          i32.load offset=8
          local.set 5
        end
        local.get 2
        i32.const 24
        i32.add
        i32.const 24
        i32.add
        local.tee 4
        local.get 1
        i32.const 24
        i32.add
        i64.load align=1
        i64.store
        local.get 2
        i32.const 24
        i32.add
        i32.const 16
        i32.add
        local.tee 6
        local.get 1
        i32.const 16
        i32.add
        i64.load align=1
        i64.store
        local.get 2
        i32.const 24
        i32.add
        i32.const 8
        i32.add
        local.tee 7
        local.get 1
        i32.const 8
        i32.add
        i64.load align=1
        i64.store
        local.get 2
        local.get 1
        i64.load align=1
        i64.store offset=24
        local.get 5
        local.get 3
        i32.add
        local.tee 1
        i32.const 3
        i32.store8
        local.get 1
        i32.const 1
        i32.add
        local.get 2
        i64.load offset=24
        i64.store align=1
        local.get 1
        i32.const 9
        i32.add
        local.get 7
        i64.load
        i64.store align=1
        local.get 1
        i32.const 17
        i32.add
        local.get 6
        i64.load
        i64.store align=1
        local.get 1
        i32.const 25
        i32.add
        local.get 4
        i64.load
        i64.store align=1
        local.get 2
        i32.const 8
        i32.add
        i32.const 8
        i32.add
        local.get 3
        i32.const 33
        i32.add
        local.tee 1
        i32.store
        local.get 0
        i32.const 8
        i32.add
        local.get 1
        i32.store
        local.get 0
        local.get 2
        i64.load offset=8
        i64.store align=4
      end
      local.get 2
      i32.const 64
      i32.add
      global.set 0
      return
    end
    unreachable
    unreachable)
  (func (;17;) (type 12) (param i32 i32)
    (local i32)
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            br_if 0 (;@4;)
            i32.const 1
            local.set 2
            br 1 (;@3;)
          end
          local.get 1
          i32.const -1
          i32.le_s
          br_if 1 (;@2;)
          i32.const 0
          i32.load8_u offset=1053220
          drop
          local.get 1
          call 4
          local.tee 2
          i32.eqz
          br_if 2 (;@1;)
        end
        local.get 0
        local.get 1
        i32.store offset=4
        local.get 0
        local.get 2
        i32.store
        return
      end
      call 29
      unreachable
    end
    unreachable
    unreachable)
  (func (;18;) (type 10) (param i32 i32 i32)
    (local i32 i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 3
    global.set 0
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        local.get 2
        i32.add
        local.tee 2
        local.get 1
        i32.lt_u
        br_if 0 (;@2;)
        local.get 0
        i32.load offset=4
        local.tee 1
        i32.const 1
        i32.shl
        local.tee 4
        local.get 2
        local.get 4
        local.get 2
        i32.gt_u
        select
        local.tee 2
        i32.const 8
        local.get 2
        i32.const 8
        i32.gt_u
        select
        local.tee 2
        i32.const -1
        i32.xor
        i32.const 31
        i32.shr_u
        local.set 4
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.eqz
            br_if 0 (;@4;)
            local.get 3
            local.get 1
            i32.store offset=24
            local.get 3
            i32.const 1
            i32.store offset=20
            local.get 3
            local.get 0
            i32.load
            i32.store offset=16
            br 1 (;@3;)
          end
          local.get 3
          i32.const 0
          i32.store offset=20
        end
        local.get 3
        local.get 4
        local.get 2
        local.get 3
        i32.const 16
        i32.add
        call 66
        local.get 3
        i32.load offset=4
        local.set 1
        block  ;; label = @3
          local.get 3
          i32.load
          br_if 0 (;@3;)
          local.get 0
          local.get 2
          i32.store offset=4
          local.get 0
          local.get 1
          i32.store
          br 2 (;@1;)
        end
        local.get 1
        i32.const -2147483647
        i32.eq
        br_if 1 (;@1;)
        local.get 1
        i32.eqz
        br_if 0 (;@2;)
        unreachable
        unreachable
      end
      call 29
      unreachable
    end
    local.get 3
    i32.const 32
    i32.add
    global.set 0)
  (func (;19;) (type 13) (param i64) (result i32)
    (local i32 i32 i32 i32 i32)
    global.get 0
    i32.const 80
    i32.sub
    local.tee 1
    global.set 0
    local.get 1
    i32.const 0
    i32.store8 offset=40
    local.get 1
    i32.const 24
    i32.add
    local.get 1
    i32.const 40
    i32.add
    call 16
    local.get 1
    i32.const 8
    i32.add
    local.get 0
    local.get 1
    i32.const 24
    i32.add
    i64.const 123456789
    call 8
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 1
          i32.load8_u offset=8
          i32.const 7
          i32.ne
          br_if 0 (;@3;)
          local.get 1
          i32.const 1
          i32.store8 offset=40
          local.get 1
          i32.const 8
          i32.add
          local.get 1
          i32.const 40
          i32.add
          call 16
          local.get 1
          i32.const 0
          i32.store offset=48
          local.get 1
          i64.const 1
          i64.store offset=40
          i32.const -8
          local.set 2
          block  ;; label = @4
            loop  ;; label = @5
              local.get 2
              i32.eqz
              br_if 1 (;@4;)
              local.get 1
              local.get 2
              i32.const 1048856
              i32.add
              i32.load8_u
              i32.store8 offset=24
              local.get 1
              i32.const 40
              i32.add
              local.get 1
              i32.const 24
              i32.add
              i32.const 1
              call 9
              local.get 2
              i32.const 1
              i32.add
              local.set 2
              br 0 (;@5;)
            end
          end
          local.get 1
          i32.load offset=48
          local.set 3
          local.get 1
          i32.load offset=44
          local.set 4
          block  ;; label = @4
            block  ;; label = @5
              local.get 1
              i32.load offset=40
              local.tee 2
              i32.eqz
              br_if 0 (;@5;)
              local.get 0
              local.get 1
              i32.load offset=8
              local.get 1
              i32.load offset=16
              local.get 2
              local.get 3
              call 2
              local.set 5
              block  ;; label = @6
                local.get 4
                i32.eqz
                br_if 0 (;@6;)
                local.get 2
                call 6
              end
              i32.const 4
              i32.const 7
              local.get 5
              select
              local.set 2
              br 1 (;@4;)
            end
            local.get 4
            call 5
            i32.const 6
            local.set 2
          end
          block  ;; label = @4
            local.get 1
            i32.load offset=12
            i32.eqz
            br_if 0 (;@4;)
            local.get 1
            i32.load offset=8
            call 6
          end
          local.get 2
          i32.const 7
          i32.ne
          br_if 1 (;@2;)
          local.get 1
          i32.const 2
          i32.store8 offset=40
          local.get 1
          i32.const 8
          i32.add
          local.get 1
          i32.const 40
          i32.add
          call 16
          local.get 1
          i32.const 0
          i32.store offset=48
          local.get 1
          i64.const 1
          i64.store offset=40
          i32.const -4
          local.set 2
          block  ;; label = @4
            loop  ;; label = @5
              local.get 2
              i32.eqz
              br_if 1 (;@4;)
              local.get 1
              local.get 2
              i32.const 1048904
              i32.add
              i32.load8_u
              i32.store8 offset=24
              local.get 1
              i32.const 40
              i32.add
              local.get 1
              i32.const 24
              i32.add
              i32.const 1
              call 9
              local.get 2
              i32.const 1
              i32.add
              local.set 2
              br 0 (;@5;)
            end
          end
          local.get 1
          i32.load offset=48
          local.set 3
          local.get 1
          i32.load offset=44
          local.set 4
          block  ;; label = @4
            block  ;; label = @5
              local.get 1
              i32.load offset=40
              local.tee 2
              i32.eqz
              br_if 0 (;@5;)
              local.get 0
              local.get 1
              i32.load offset=8
              local.get 1
              i32.load offset=16
              local.get 2
              local.get 3
              call 2
              local.set 5
              block  ;; label = @6
                local.get 4
                i32.eqz
                br_if 0 (;@6;)
                local.get 2
                call 6
              end
              i32.const 4
              i32.const 7
              local.get 5
              select
              local.set 2
              br 1 (;@4;)
            end
            local.get 4
            call 5
            i32.const 6
            local.set 2
          end
          block  ;; label = @4
            local.get 1
            i32.load offset=12
            i32.eqz
            br_if 0 (;@4;)
            local.get 1
            i32.load offset=8
            call 6
          end
          local.get 2
          i32.const 7
          i32.ne
          br_if 2 (;@1;)
          local.get 1
          i32.const 80
          i32.add
          global.set 0
          i32.const 1
          return
        end
        local.get 1
        i32.const 40
        i32.add
        i32.const 8
        i32.add
        local.get 1
        i32.const 8
        i32.add
        i32.const 8
        i32.add
        i64.load
        i64.store
        local.get 1
        local.get 1
        i64.load offset=8
        i64.store offset=40
        i32.const 1048760
        i32.const 28
        local.get 1
        i32.const 40
        i32.add
        i32.const 1048712
        i32.const 1048832
        call 7
        unreachable
      end
      local.get 1
      local.get 3
      i32.store offset=52
      local.get 1
      local.get 4
      i32.store offset=48
      local.get 1
      i32.const 0
      i32.store offset=44
      local.get 1
      local.get 2
      i32.store8 offset=40
      i32.const 1048856
      i32.const 25
      local.get 1
      i32.const 40
      i32.add
      i32.const 1048712
      i32.const 1048884
      call 7
      unreachable
    end
    local.get 1
    local.get 3
    i32.store offset=52
    local.get 1
    local.get 4
    i32.store offset=48
    local.get 1
    i32.const 0
    i32.store offset=44
    local.get 1
    local.get 2
    i32.store8 offset=40
    i32.const 1048904
    i32.const 22
    local.get 1
    i32.const 40
    i32.add
    i32.const 1048712
    i32.const 1048928
    call 7
    unreachable)
  (func (;20;) (type 14) (param i64) (result i64)
    (local i32)
    global.get 0
    i32.const 80
    i32.sub
    local.tee 1
    global.set 0
    local.get 1
    i32.const 0
    i32.store8 offset=40
    local.get 1
    i32.const 24
    i32.add
    local.get 1
    i32.const 40
    i32.add
    call 16
    local.get 1
    i32.const 8
    i32.add
    local.get 0
    local.get 1
    i32.const 24
    i32.add
    call 3
    block  ;; label = @1
      local.get 1
      i32.load8_u offset=8
      i32.const 7
      i32.eq
      br_if 0 (;@1;)
      local.get 1
      i32.const 40
      i32.add
      i32.const 8
      i32.add
      local.get 1
      i32.const 8
      i32.add
      i32.const 8
      i32.add
      i64.load
      i64.store
      local.get 1
      local.get 1
      i64.load offset=8
      i64.store offset=40
      i32.const 1048944
      i32.const 26
      local.get 1
      i32.const 40
      i32.add
      i32.const 1048712
      i32.const 1048972
      call 7
      unreachable
    end
    local.get 1
    i64.load offset=16
    local.set 0
    local.get 1
    i32.const 80
    i32.add
    global.set 0
    local.get 0)
  (func (;21;) (type 15) (param i64 i64 i64) (result i32)
    (local i32 i32 i32 i32 i32)
    global.get 0
    i32.const 112
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    i32.const 8
    i32.add
    i32.const 24
    i32.add
    local.get 1
    i32.wrap_i64
    local.tee 4
    i32.const 24
    i32.add
    local.tee 5
    i64.load align=1
    i64.store
    local.get 3
    i32.const 8
    i32.add
    i32.const 16
    i32.add
    local.get 4
    i32.const 16
    i32.add
    local.tee 6
    i64.load align=1
    i64.store
    local.get 3
    i32.const 8
    i32.add
    i32.const 8
    i32.add
    local.get 4
    i32.const 8
    i32.add
    local.tee 7
    i64.load align=1
    i64.store
    local.get 3
    local.get 4
    i64.load align=1
    i64.store offset=8
    local.get 3
    i32.const 97
    i32.add
    local.get 5
    i64.load align=1
    i64.store align=1
    local.get 3
    i32.const 89
    i32.add
    local.get 6
    i64.load align=1
    i64.store align=1
    local.get 3
    i32.const 81
    i32.add
    local.get 7
    i64.load align=1
    i64.store align=1
    local.get 3
    i32.const 3
    i32.store8 offset=72
    local.get 3
    local.get 4
    i64.load align=1
    i64.store offset=73 align=1
    local.get 3
    i32.const 40
    i32.add
    local.get 3
    i32.const 72
    i32.add
    call 16
    local.get 3
    i32.const 72
    i32.add
    local.get 0
    local.get 3
    i32.const 40
    i32.add
    call 3
    block  ;; label = @1
      block  ;; label = @2
        local.get 3
        i32.load8_u offset=72
        local.tee 4
        i32.const 7
        i32.ne
        br_if 0 (;@2;)
        local.get 3
        i64.load offset=80
        local.set 1
        br 1 (;@1;)
      end
      i64.const 0
      local.set 1
      local.get 4
      br_if 0 (;@1;)
      local.get 3
      i32.const 80
      i32.add
      i32.load
      i32.eqz
      br_if 0 (;@1;)
      local.get 3
      i32.load offset=76
      call 6
    end
    local.get 3
    i32.const 97
    i32.add
    local.get 3
    i32.const 32
    i32.add
    i64.load
    i64.store align=1
    local.get 3
    i32.const 89
    i32.add
    local.get 3
    i32.const 24
    i32.add
    i64.load
    i64.store align=1
    local.get 3
    i32.const 81
    i32.add
    local.get 3
    i32.const 16
    i32.add
    i64.load
    i64.store align=1
    local.get 3
    local.get 3
    i64.load offset=8
    i64.store offset=73 align=1
    local.get 3
    i32.const 3
    i32.store8 offset=72
    local.get 3
    i32.const 56
    i32.add
    local.get 3
    i32.const 72
    i32.add
    call 16
    block  ;; label = @1
      block  ;; label = @2
        local.get 2
        i64.const 0
        i64.lt_s
        local.get 1
        local.get 2
        i64.add
        local.tee 2
        local.get 1
        i64.lt_s
        i32.xor
        br_if 0 (;@2;)
        local.get 3
        i32.const 40
        i32.add
        local.get 0
        local.get 3
        i32.const 56
        i32.add
        local.get 2
        call 8
        local.get 3
        i32.load8_u offset=40
        i32.const 7
        i32.eq
        br_if 1 (;@1;)
        local.get 3
        i32.const 72
        i32.add
        i32.const 8
        i32.add
        local.get 3
        i32.const 40
        i32.add
        i32.const 8
        i32.add
        i64.load
        i64.store
        local.get 3
        local.get 3
        i64.load offset=40
        i64.store offset=72
        i32.const 1049036
        i32.const 23
        local.get 3
        i32.const 72
        i32.add
        i32.const 1048712
        i32.const 1049060
        call 7
        unreachable
      end
      i32.const 1049008
      i32.const 28
      i32.const 1048988
      call 22
      unreachable
    end
    local.get 3
    i32.const 112
    i32.add
    global.set 0
    i32.const 1)
  (func (;22;) (type 10) (param i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    i32.const 12
    i32.add
    i64.const 0
    i64.store align=4
    local.get 3
    i32.const 1
    i32.store offset=4
    local.get 3
    i32.const 1053004
    i32.store offset=8
    local.get 3
    local.get 1
    i32.store offset=28
    local.get 3
    local.get 0
    i32.store offset=24
    local.get 3
    local.get 3
    i32.const 24
    i32.add
    i32.store
    local.get 3
    local.get 2
    call 24
    unreachable)
  (func (;23;) (type 16) (param i64 i64 i64 i64) (result i32)
    (local i32 i32 i32 i32 i32 i64)
    global.get 0
    i32.const 144
    i32.sub
    local.tee 4
    global.set 0
    local.get 4
    i32.const 8
    i32.add
    i32.const 24
    i32.add
    local.tee 5
    local.get 1
    i32.wrap_i64
    local.tee 6
    i32.const 24
    i32.add
    i64.load align=1
    i64.store
    local.get 4
    i32.const 8
    i32.add
    i32.const 16
    i32.add
    local.tee 7
    local.get 6
    i32.const 16
    i32.add
    i64.load align=1
    i64.store
    local.get 4
    i32.const 8
    i32.add
    i32.const 8
    i32.add
    local.tee 8
    local.get 6
    i32.const 8
    i32.add
    i64.load align=1
    i64.store
    local.get 4
    local.get 6
    i64.load align=1
    i64.store offset=8
    local.get 4
    i32.const 40
    i32.add
    i32.const 24
    i32.add
    local.get 2
    i32.wrap_i64
    local.tee 6
    i32.const 24
    i32.add
    i64.load align=1
    i64.store
    local.get 4
    i32.const 40
    i32.add
    i32.const 16
    i32.add
    local.get 6
    i32.const 16
    i32.add
    i64.load align=1
    i64.store
    local.get 4
    i32.const 40
    i32.add
    i32.const 8
    i32.add
    local.get 6
    i32.const 8
    i32.add
    i64.load align=1
    i64.store
    local.get 4
    local.get 6
    i64.load align=1
    i64.store offset=40
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 4
            i32.const 8
            i32.add
            local.get 4
            i32.const 40
            i32.add
            i32.const 32
            call 82
            i32.eqz
            br_if 0 (;@4;)
            local.get 4
            i32.const 129
            i32.add
            local.get 5
            i64.load
            i64.store align=1
            local.get 4
            i32.const 121
            i32.add
            local.get 7
            i64.load
            i64.store align=1
            local.get 4
            i32.const 113
            i32.add
            local.get 8
            i64.load
            i64.store align=1
            local.get 4
            local.get 4
            i64.load offset=8
            i64.store offset=105 align=1
            local.get 4
            i32.const 3
            i32.store8 offset=104
            local.get 4
            i32.const 88
            i32.add
            local.get 4
            i32.const 104
            i32.add
            call 16
            local.get 4
            i32.const 72
            i32.add
            local.get 0
            local.get 4
            i32.const 88
            i32.add
            call 3
            block  ;; label = @5
              local.get 4
              i32.load8_u offset=72
              i32.const 7
              i32.ne
              br_if 0 (;@5;)
              local.get 3
              i64.const 0
              i64.lt_s
              br_if 2 (;@3;)
              local.get 4
              i64.load offset=80
              local.tee 1
              local.get 3
              i64.lt_s
              br_if 2 (;@3;)
              local.get 4
              i32.const 129
              i32.add
              local.get 4
              i32.const 64
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              i32.const 121
              i32.add
              local.get 4
              i32.const 56
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              i32.const 113
              i32.add
              local.get 4
              i32.const 48
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              local.get 4
              i64.load offset=40
              i64.store offset=105 align=1
              local.get 4
              i32.const 3
              i32.store8 offset=104
              local.get 4
              i32.const 72
              i32.add
              local.get 4
              i32.const 104
              i32.add
              call 16
              local.get 4
              i32.const 104
              i32.add
              local.get 0
              local.get 4
              i32.const 72
              i32.add
              call 3
              block  ;; label = @6
                block  ;; label = @7
                  local.get 4
                  i32.load8_u offset=104
                  local.tee 6
                  i32.const 7
                  i32.ne
                  br_if 0 (;@7;)
                  local.get 4
                  i64.load offset=112
                  local.set 2
                  br 1 (;@6;)
                end
                i64.const 0
                local.set 2
                local.get 6
                br_if 0 (;@6;)
                local.get 4
                i32.const 112
                i32.add
                i32.load
                i32.eqz
                br_if 0 (;@6;)
                local.get 4
                i32.load offset=108
                call 6
              end
              local.get 4
              i32.const 129
              i32.add
              local.get 4
              i32.const 32
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              i32.const 121
              i32.add
              local.get 4
              i32.const 24
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              i32.const 113
              i32.add
              local.get 4
              i32.const 16
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              local.get 4
              i64.load offset=8
              i64.store offset=105 align=1
              local.get 4
              i32.const 3
              i32.store8 offset=104
              local.get 4
              i32.const 88
              i32.add
              local.get 4
              i32.const 104
              i32.add
              call 16
              local.get 3
              i64.const 0
              i64.gt_s
              local.get 1
              local.get 3
              i64.sub
              local.tee 9
              local.get 1
              i64.lt_s
              i32.xor
              br_if 4 (;@1;)
              local.get 4
              i32.const 72
              i32.add
              local.get 0
              local.get 4
              i32.const 88
              i32.add
              local.get 9
              call 8
              local.get 4
              i32.load8_u offset=72
              i32.const 7
              i32.ne
              br_if 3 (;@2;)
              local.get 4
              i32.const 129
              i32.add
              local.get 4
              i32.const 64
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              i32.const 121
              i32.add
              local.get 4
              i32.const 56
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              i32.const 113
              i32.add
              local.get 4
              i32.const 48
              i32.add
              i64.load
              i64.store align=1
              local.get 4
              local.get 4
              i64.load offset=40
              i64.store offset=105 align=1
              local.get 4
              i32.const 3
              i32.store8 offset=104
              local.get 4
              i32.const 88
              i32.add
              local.get 4
              i32.const 104
              i32.add
              call 16
              block  ;; label = @6
                block  ;; label = @7
                  local.get 3
                  i64.const 0
                  i64.lt_s
                  local.get 2
                  local.get 3
                  i64.add
                  local.tee 3
                  local.get 2
                  i64.lt_s
                  i32.xor
                  br_if 0 (;@7;)
                  local.get 4
                  i32.const 72
                  i32.add
                  local.get 0
                  local.get 4
                  i32.const 88
                  i32.add
                  local.get 3
                  call 8
                  local.get 4
                  i32.load8_u offset=72
                  i32.const 7
                  i32.eq
                  br_if 1 (;@6;)
                  local.get 4
                  i32.const 104
                  i32.add
                  i32.const 8
                  i32.add
                  local.get 4
                  i32.const 72
                  i32.add
                  i32.const 8
                  i32.add
                  i64.load
                  i64.store
                  local.get 4
                  local.get 4
                  i64.load offset=72
                  i64.store offset=104
                  i32.const 1049036
                  i32.const 23
                  local.get 4
                  i32.const 104
                  i32.add
                  i32.const 1048712
                  i32.const 1049204
                  call 7
                  unreachable
                end
                i32.const 1049008
                i32.const 28
                i32.const 1049188
                call 22
                unreachable
              end
              local.get 4
              i32.const 144
              i32.add
              global.set 0
              i32.const 1
              return
            end
            local.get 4
            i32.const 104
            i32.add
            i32.const 8
            i32.add
            local.get 4
            i32.const 72
            i32.add
            i32.const 8
            i32.add
            i64.load
            i64.store
            local.get 4
            local.get 4
            i64.load offset=72
            i64.store offset=104
            i32.const 1049076
            i32.const 24
            local.get 4
            i32.const 104
            i32.add
            i32.const 1048712
            i32.const 1049100
            call 7
            unreachable
          end
          local.get 4
          i64.const 0
          i64.store offset=116 align=4
          local.get 4
          i32.const 1053004
          i32.store offset=112
          local.get 4
          i32.const 1
          i32.store offset=108
          local.get 4
          i32.const 1049300
          i32.store offset=104
          local.get 4
          i32.const 8
          i32.add
          local.get 4
          i32.const 40
          i32.add
          local.get 4
          i32.const 104
          i32.add
          call 14
          unreachable
        end
        local.get 4
        i32.const 116
        i32.add
        i64.const 0
        i64.store align=4
        local.get 4
        i32.const 1
        i32.store offset=108
        local.get 4
        i32.const 1049236
        i32.store offset=104
        local.get 4
        i32.const 1053004
        i32.store offset=112
        local.get 4
        i32.const 104
        i32.add
        i32.const 1049244
        call 24
        unreachable
      end
      local.get 4
      i32.const 104
      i32.add
      i32.const 8
      i32.add
      local.get 4
      i32.const 72
      i32.add
      i32.const 8
      i32.add
      i64.load
      i64.store
      local.get 4
      local.get 4
      i64.load offset=72
      i64.store offset=104
      i32.const 1049036
      i32.const 23
      local.get 4
      i32.const 104
      i32.add
      i32.const 1048712
      i32.const 1049172
      call 7
      unreachable
    end
    i32.const 1049136
    i32.const 33
    i32.const 1049116
    call 22
    unreachable)
  (func (;24;) (type 12) (param i32 i32)
    (local i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    local.get 0
    i32.store offset=20
    local.get 2
    i32.const 1049440
    i32.store offset=12
    local.get 2
    i32.const 1053004
    i32.store offset=8
    local.get 2
    i32.const 1
    i32.store8 offset=24
    local.get 2
    local.get 1
    i32.store offset=16
    local.get 2
    i32.const 8
    i32.add
    call 31
    unreachable)
  (func (;25;) (type 17) (param i64 i64) (result i64)
    (local i32 i32)
    global.get 0
    i32.const 64
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    i32.const 49
    i32.add
    local.get 1
    i32.wrap_i64
    local.tee 3
    i32.const 24
    i32.add
    i64.load align=1
    i64.store align=1
    local.get 2
    i32.const 41
    i32.add
    local.get 3
    i32.const 16
    i32.add
    i64.load align=1
    i64.store align=1
    local.get 2
    i32.const 33
    i32.add
    local.get 3
    i32.const 8
    i32.add
    i64.load align=1
    i64.store align=1
    local.get 2
    local.get 3
    i64.load align=1
    i64.store offset=25 align=1
    local.get 2
    i32.const 3
    i32.store8 offset=24
    local.get 2
    i32.const 8
    i32.add
    local.get 2
    i32.const 24
    i32.add
    call 16
    local.get 2
    i32.const 24
    i32.add
    local.get 0
    local.get 2
    i32.const 8
    i32.add
    call 3
    block  ;; label = @1
      block  ;; label = @2
        local.get 2
        i32.load8_u offset=24
        local.tee 3
        i32.const 7
        i32.ne
        br_if 0 (;@2;)
        local.get 2
        i64.load offset=32
        local.set 1
        br 1 (;@1;)
      end
      i64.const 0
      local.set 1
      local.get 3
      br_if 0 (;@1;)
      local.get 2
      i32.const 32
      i32.add
      i32.load
      i32.eqz
      br_if 0 (;@1;)
      local.get 2
      i32.load offset=28
      call 6
    end
    local.get 2
    i32.const 64
    i32.add
    global.set 0
    local.get 1)
  (func (;26;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32)
    i32.const 0
    local.set 2
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.const -65588
        i32.gt_u
        br_if 0 (;@2;)
        i32.const 16
        local.get 1
        i32.const 11
        i32.add
        i32.const -8
        i32.and
        local.get 1
        i32.const 11
        i32.lt_u
        select
        local.set 3
        local.get 0
        i32.const -4
        i32.add
        local.tee 4
        i32.load
        local.tee 5
        i32.const -8
        i32.and
        local.set 6
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 5
                        i32.const 3
                        i32.and
                        i32.eqz
                        br_if 0 (;@10;)
                        local.get 0
                        i32.const -8
                        i32.add
                        local.set 7
                        local.get 6
                        local.get 3
                        i32.ge_u
                        br_if 1 (;@9;)
                        local.get 7
                        local.get 6
                        i32.add
                        local.tee 8
                        i32.const 0
                        i32.load offset=1053660
                        i32.eq
                        br_if 6 (;@4;)
                        local.get 8
                        i32.const 0
                        i32.load offset=1053656
                        i32.eq
                        br_if 4 (;@6;)
                        local.get 8
                        i32.load offset=4
                        local.tee 5
                        i32.const 2
                        i32.and
                        br_if 7 (;@3;)
                        local.get 5
                        i32.const -8
                        i32.and
                        local.tee 9
                        local.get 6
                        i32.add
                        local.tee 6
                        local.get 3
                        i32.lt_u
                        br_if 7 (;@3;)
                        local.get 6
                        local.get 3
                        i32.sub
                        local.set 1
                        local.get 9
                        i32.const 256
                        i32.lt_u
                        br_if 2 (;@8;)
                        local.get 8
                        call 27
                        br 3 (;@7;)
                      end
                      local.get 3
                      i32.const 256
                      i32.lt_u
                      br_if 6 (;@3;)
                      local.get 6
                      local.get 3
                      i32.const 4
                      i32.or
                      i32.lt_u
                      br_if 6 (;@3;)
                      local.get 6
                      local.get 3
                      i32.sub
                      i32.const 131073
                      i32.ge_u
                      br_if 6 (;@3;)
                      local.get 0
                      return
                    end
                    local.get 6
                    local.get 3
                    i32.sub
                    local.tee 1
                    i32.const 16
                    i32.ge_u
                    br_if 3 (;@5;)
                    local.get 0
                    return
                  end
                  block  ;; label = @8
                    local.get 8
                    i32.const 12
                    i32.add
                    i32.load
                    local.tee 2
                    local.get 8
                    i32.const 8
                    i32.add
                    i32.load
                    local.tee 8
                    i32.eq
                    br_if 0 (;@8;)
                    local.get 8
                    local.get 2
                    i32.store offset=12
                    local.get 2
                    local.get 8
                    i32.store offset=8
                    br 1 (;@7;)
                  end
                  i32.const 0
                  i32.const 0
                  i32.load offset=1053640
                  i32.const -2
                  local.get 5
                  i32.const 3
                  i32.shr_u
                  i32.rotl
                  i32.and
                  i32.store offset=1053640
                end
                block  ;; label = @7
                  local.get 1
                  i32.const 16
                  i32.lt_u
                  br_if 0 (;@7;)
                  local.get 4
                  local.get 4
                  i32.load
                  i32.const 1
                  i32.and
                  local.get 3
                  i32.or
                  i32.const 2
                  i32.or
                  i32.store
                  local.get 7
                  local.get 3
                  i32.add
                  local.tee 2
                  local.get 1
                  i32.const 3
                  i32.or
                  i32.store offset=4
                  local.get 2
                  local.get 1
                  i32.add
                  local.tee 3
                  local.get 3
                  i32.load offset=4
                  i32.const 1
                  i32.or
                  i32.store offset=4
                  local.get 2
                  local.get 1
                  call 28
                  local.get 0
                  return
                end
                local.get 4
                local.get 4
                i32.load
                i32.const 1
                i32.and
                local.get 6
                i32.or
                i32.const 2
                i32.or
                i32.store
                local.get 7
                local.get 6
                i32.add
                local.tee 1
                local.get 1
                i32.load offset=4
                i32.const 1
                i32.or
                i32.store offset=4
                local.get 0
                return
              end
              i32.const 0
              i32.load offset=1053648
              local.get 6
              i32.add
              local.tee 6
              local.get 3
              i32.lt_u
              br_if 2 (;@3;)
              block  ;; label = @6
                block  ;; label = @7
                  local.get 6
                  local.get 3
                  i32.sub
                  local.tee 1
                  i32.const 15
                  i32.gt_u
                  br_if 0 (;@7;)
                  local.get 4
                  local.get 5
                  i32.const 1
                  i32.and
                  local.get 6
                  i32.or
                  i32.const 2
                  i32.or
                  i32.store
                  local.get 7
                  local.get 6
                  i32.add
                  local.tee 1
                  local.get 1
                  i32.load offset=4
                  i32.const 1
                  i32.or
                  i32.store offset=4
                  i32.const 0
                  local.set 1
                  i32.const 0
                  local.set 2
                  br 1 (;@6;)
                end
                local.get 4
                local.get 5
                i32.const 1
                i32.and
                local.get 3
                i32.or
                i32.const 2
                i32.or
                i32.store
                local.get 7
                local.get 3
                i32.add
                local.tee 2
                local.get 1
                i32.const 1
                i32.or
                i32.store offset=4
                local.get 2
                local.get 1
                i32.add
                local.tee 3
                local.get 1
                i32.store
                local.get 3
                local.get 3
                i32.load offset=4
                i32.const -2
                i32.and
                i32.store offset=4
              end
              i32.const 0
              local.get 2
              i32.store offset=1053656
              i32.const 0
              local.get 1
              i32.store offset=1053648
              local.get 0
              return
            end
            local.get 4
            local.get 5
            i32.const 1
            i32.and
            local.get 3
            i32.or
            i32.const 2
            i32.or
            i32.store
            local.get 7
            local.get 3
            i32.add
            local.tee 2
            local.get 1
            i32.const 3
            i32.or
            i32.store offset=4
            local.get 2
            local.get 1
            i32.add
            local.tee 3
            local.get 3
            i32.load offset=4
            i32.const 1
            i32.or
            i32.store offset=4
            local.get 2
            local.get 1
            call 28
            local.get 0
            return
          end
          i32.const 0
          i32.load offset=1053652
          local.get 6
          i32.add
          local.tee 6
          local.get 3
          i32.gt_u
          br_if 2 (;@1;)
        end
        local.get 1
        call 4
        local.tee 3
        i32.eqz
        br_if 0 (;@2;)
        local.get 3
        local.get 0
        i32.const -4
        i32.const -8
        local.get 4
        i32.load
        local.tee 2
        i32.const 3
        i32.and
        select
        local.get 2
        i32.const -8
        i32.and
        i32.add
        local.tee 2
        local.get 1
        local.get 2
        local.get 1
        i32.lt_u
        select
        call 81
        local.set 1
        local.get 0
        call 6
        local.get 1
        local.set 2
      end
      local.get 2
      return
    end
    local.get 4
    local.get 5
    i32.const 1
    i32.and
    local.get 3
    i32.or
    i32.const 2
    i32.or
    i32.store
    local.get 7
    local.get 3
    i32.add
    local.tee 1
    local.get 6
    local.get 3
    i32.sub
    local.tee 2
    i32.const 1
    i32.or
    i32.store offset=4
    i32.const 0
    local.get 2
    i32.store offset=1053652
    i32.const 0
    local.get 1
    i32.store offset=1053660
    local.get 0)
  (func (;27;) (type 0) (param i32)
    (local i32 i32 i32 i32 i32)
    local.get 0
    i32.load offset=24
    local.set 1
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 0
          i32.load offset=12
          local.tee 2
          local.get 0
          i32.ne
          br_if 0 (;@3;)
          local.get 0
          i32.const 20
          i32.const 16
          local.get 0
          i32.const 20
          i32.add
          local.tee 2
          i32.load
          local.tee 3
          select
          i32.add
          i32.load
          local.tee 4
          br_if 1 (;@2;)
          i32.const 0
          local.set 2
          br 2 (;@1;)
        end
        local.get 0
        i32.load offset=8
        local.tee 4
        local.get 2
        i32.store offset=12
        local.get 2
        local.get 4
        i32.store offset=8
        br 1 (;@1;)
      end
      local.get 2
      local.get 0
      i32.const 16
      i32.add
      local.get 3
      select
      local.set 3
      loop  ;; label = @2
        local.get 3
        local.set 5
        local.get 4
        local.tee 2
        i32.const 20
        i32.add
        local.tee 4
        local.get 2
        i32.const 16
        i32.add
        local.get 4
        i32.load
        local.tee 4
        select
        local.set 3
        local.get 2
        i32.const 20
        i32.const 16
        local.get 4
        select
        i32.add
        i32.load
        local.tee 4
        br_if 0 (;@2;)
      end
      local.get 5
      i32.const 0
      i32.store
    end
    block  ;; label = @1
      local.get 1
      i32.eqz
      br_if 0 (;@1;)
      block  ;; label = @2
        block  ;; label = @3
          local.get 0
          i32.load offset=28
          i32.const 2
          i32.shl
          i32.const 1053232
          i32.add
          local.tee 4
          i32.load
          local.get 0
          i32.eq
          br_if 0 (;@3;)
          local.get 1
          i32.const 16
          i32.const 20
          local.get 1
          i32.load offset=16
          local.get 0
          i32.eq
          select
          i32.add
          local.get 2
          i32.store
          local.get 2
          br_if 1 (;@2;)
          br 2 (;@1;)
        end
        local.get 4
        local.get 2
        i32.store
        local.get 2
        br_if 0 (;@2;)
        i32.const 0
        i32.const 0
        i32.load offset=1053644
        i32.const -2
        local.get 0
        i32.load offset=28
        i32.rotl
        i32.and
        i32.store offset=1053644
        return
      end
      local.get 2
      local.get 1
      i32.store offset=24
      block  ;; label = @2
        local.get 0
        i32.load offset=16
        local.tee 4
        i32.eqz
        br_if 0 (;@2;)
        local.get 2
        local.get 4
        i32.store offset=16
        local.get 4
        local.get 2
        i32.store offset=24
      end
      local.get 0
      i32.const 20
      i32.add
      i32.load
      local.tee 4
      i32.eqz
      br_if 0 (;@1;)
      local.get 2
      i32.const 20
      i32.add
      local.get 4
      i32.store
      local.get 4
      local.get 2
      i32.store offset=24
      return
    end)
  (func (;28;) (type 12) (param i32 i32)
    (local i32 i32 i32 i32)
    local.get 0
    local.get 1
    i32.add
    local.set 2
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 0
          i32.load offset=4
          local.tee 3
          i32.const 1
          i32.and
          br_if 0 (;@3;)
          local.get 3
          i32.const 3
          i32.and
          i32.eqz
          br_if 1 (;@2;)
          local.get 0
          i32.load
          local.tee 3
          local.get 1
          i32.add
          local.set 1
          block  ;; label = @4
            local.get 0
            local.get 3
            i32.sub
            local.tee 0
            i32.const 0
            i32.load offset=1053656
            i32.ne
            br_if 0 (;@4;)
            local.get 2
            i32.load offset=4
            i32.const 3
            i32.and
            i32.const 3
            i32.ne
            br_if 1 (;@3;)
            i32.const 0
            local.get 1
            i32.store offset=1053648
            local.get 2
            local.get 2
            i32.load offset=4
            i32.const -2
            i32.and
            i32.store offset=4
            local.get 0
            local.get 1
            i32.const 1
            i32.or
            i32.store offset=4
            local.get 2
            local.get 1
            i32.store
            return
          end
          block  ;; label = @4
            local.get 3
            i32.const 256
            i32.lt_u
            br_if 0 (;@4;)
            local.get 0
            call 27
            br 1 (;@3;)
          end
          block  ;; label = @4
            local.get 0
            i32.const 12
            i32.add
            i32.load
            local.tee 4
            local.get 0
            i32.const 8
            i32.add
            i32.load
            local.tee 5
            i32.eq
            br_if 0 (;@4;)
            local.get 5
            local.get 4
            i32.store offset=12
            local.get 4
            local.get 5
            i32.store offset=8
            br 1 (;@3;)
          end
          i32.const 0
          i32.const 0
          i32.load offset=1053640
          i32.const -2
          local.get 3
          i32.const 3
          i32.shr_u
          i32.rotl
          i32.and
          i32.store offset=1053640
        end
        block  ;; label = @3
          local.get 2
          i32.load offset=4
          local.tee 3
          i32.const 2
          i32.and
          i32.eqz
          br_if 0 (;@3;)
          local.get 2
          local.get 3
          i32.const -2
          i32.and
          i32.store offset=4
          local.get 0
          local.get 1
          i32.const 1
          i32.or
          i32.store offset=4
          local.get 0
          local.get 1
          i32.add
          local.get 1
          i32.store
          br 2 (;@1;)
        end
        block  ;; label = @3
          block  ;; label = @4
            local.get 2
            i32.const 0
            i32.load offset=1053660
            i32.eq
            br_if 0 (;@4;)
            local.get 2
            i32.const 0
            i32.load offset=1053656
            i32.eq
            br_if 1 (;@3;)
            local.get 3
            i32.const -8
            i32.and
            local.tee 4
            local.get 1
            i32.add
            local.set 1
            block  ;; label = @5
              block  ;; label = @6
                local.get 4
                i32.const 256
                i32.lt_u
                br_if 0 (;@6;)
                local.get 2
                call 27
                br 1 (;@5;)
              end
              block  ;; label = @6
                local.get 2
                i32.const 12
                i32.add
                i32.load
                local.tee 4
                local.get 2
                i32.const 8
                i32.add
                i32.load
                local.tee 2
                i32.eq
                br_if 0 (;@6;)
                local.get 2
                local.get 4
                i32.store offset=12
                local.get 4
                local.get 2
                i32.store offset=8
                br 1 (;@5;)
              end
              i32.const 0
              i32.const 0
              i32.load offset=1053640
              i32.const -2
              local.get 3
              i32.const 3
              i32.shr_u
              i32.rotl
              i32.and
              i32.store offset=1053640
            end
            local.get 0
            local.get 1
            i32.const 1
            i32.or
            i32.store offset=4
            local.get 0
            local.get 1
            i32.add
            local.get 1
            i32.store
            local.get 0
            i32.const 0
            i32.load offset=1053656
            i32.ne
            br_if 3 (;@1;)
            i32.const 0
            local.get 1
            i32.store offset=1053648
            br 2 (;@2;)
          end
          i32.const 0
          local.get 0
          i32.store offset=1053660
          i32.const 0
          i32.const 0
          i32.load offset=1053652
          local.get 1
          i32.add
          local.tee 1
          i32.store offset=1053652
          local.get 0
          local.get 1
          i32.const 1
          i32.or
          i32.store offset=4
          local.get 0
          i32.const 0
          i32.load offset=1053656
          i32.ne
          br_if 1 (;@2;)
          i32.const 0
          i32.const 0
          i32.store offset=1053648
          i32.const 0
          i32.const 0
          i32.store offset=1053656
          return
        end
        i32.const 0
        local.get 0
        i32.store offset=1053656
        i32.const 0
        i32.const 0
        i32.load offset=1053648
        local.get 1
        i32.add
        local.tee 1
        i32.store offset=1053648
        local.get 0
        local.get 1
        i32.const 1
        i32.or
        i32.store offset=4
        local.get 0
        local.get 1
        i32.add
        local.get 1
        i32.store
        return
      end
      return
    end
    block  ;; label = @1
      local.get 1
      i32.const 256
      i32.lt_u
      br_if 0 (;@1;)
      local.get 0
      local.get 1
      call 68
      return
    end
    local.get 1
    i32.const -8
    i32.and
    i32.const 1053376
    i32.add
    local.set 2
    block  ;; label = @1
      block  ;; label = @2
        i32.const 0
        i32.load offset=1053640
        local.tee 3
        i32.const 1
        local.get 1
        i32.const 3
        i32.shr_u
        i32.shl
        local.tee 1
        i32.and
        i32.eqz
        br_if 0 (;@2;)
        local.get 2
        i32.load offset=8
        local.set 1
        br 1 (;@1;)
      end
      i32.const 0
      local.get 3
      local.get 1
      i32.or
      i32.store offset=1053640
      local.get 2
      local.set 1
    end
    local.get 2
    local.get 0
    i32.store offset=8
    local.get 1
    local.get 0
    i32.store offset=12
    local.get 0
    local.get 2
    i32.store offset=12
    local.get 0
    local.get 1
    i32.store offset=8)
  (func (;29;) (type 18)
    (local i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 0
    global.set 0
    local.get 0
    i32.const 20
    i32.add
    i64.const 0
    i64.store align=4
    local.get 0
    i32.const 1
    i32.store offset=12
    local.get 0
    i32.const 1049372
    i32.store offset=8
    local.get 0
    i32.const 1053004
    i32.store offset=16
    local.get 0
    i32.const 8
    i32.add
    i32.const 1049380
    call 24
    unreachable)
  (func (;30;) (type 2) (param i32 i32) (result i32)
    local.get 0
    i32.load
    drop
    loop (result i32)  ;; label = @1
      br 0 (;@1;)
    end)
  (func (;31;) (type 0) (param i32)
    (local i32 i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 1
    global.set 0
    block  ;; label = @1
      local.get 0
      i32.load offset=12
      local.tee 2
      br_if 0 (;@1;)
      i32.const 1052959
      i32.const 43
      i32.const 1053032
      call 22
      unreachable
    end
    local.get 1
    local.get 0
    i32.load offset=8
    i32.store offset=8
    local.get 1
    local.get 0
    i32.store offset=4
    local.get 1
    local.get 2
    i32.store
    local.get 1
    call 69
    unreachable)
  (func (;32;) (type 0) (param i32))
  (func (;33;) (type 12) (param i32 i32)
    local.get 0
    i64.const 6709583872402221221
    i64.store offset=8
    local.get 0
    i64.const -517914840449640987
    i64.store)
  (func (;34;) (type 10) (param i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 48
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    local.get 1
    i32.store offset=4
    local.get 3
    local.get 0
    i32.store
    local.get 3
    i32.const 8
    i32.add
    i32.const 12
    i32.add
    i64.const 2
    i64.store align=4
    local.get 3
    i32.const 32
    i32.add
    i32.const 12
    i32.add
    i32.const 4
    i32.store
    local.get 3
    i32.const 2
    i32.store offset=12
    local.get 3
    i32.const 1049508
    i32.store offset=8
    local.get 3
    i32.const 4
    i32.store offset=36
    local.get 3
    local.get 3
    i32.const 32
    i32.add
    i32.store offset=16
    local.get 3
    local.get 3
    i32.store offset=40
    local.get 3
    local.get 3
    i32.const 4
    i32.add
    i32.store offset=32
    local.get 3
    i32.const 8
    i32.add
    local.get 2
    call 24
    unreachable)
  (func (;35;) (type 2) (param i32 i32) (result i32)
    local.get 0
    i64.load32_u
    local.get 1
    call 36)
  (func (;36;) (type 19) (param i64 i32) (result i32)
    (local i32 i32 i64 i32 i32 i32)
    global.get 0
    i32.const 48
    i32.sub
    local.tee 2
    global.set 0
    i32.const 39
    local.set 3
    block  ;; label = @1
      block  ;; label = @2
        local.get 0
        i64.const 10000
        i64.ge_u
        br_if 0 (;@2;)
        local.get 0
        local.set 4
        br 1 (;@1;)
      end
      i32.const 39
      local.set 3
      loop  ;; label = @2
        local.get 2
        i32.const 9
        i32.add
        local.get 3
        i32.add
        local.tee 5
        i32.const -4
        i32.add
        local.get 0
        i64.const 10000
        i64.div_u
        local.tee 4
        i64.const 55536
        i64.mul
        local.get 0
        i64.add
        i32.wrap_i64
        local.tee 6
        i32.const 65535
        i32.and
        i32.const 100
        i32.div_u
        local.tee 7
        i32.const 1
        i32.shl
        i32.const 1049758
        i32.add
        i32.load16_u align=1
        i32.store16 align=1
        local.get 5
        i32.const -2
        i32.add
        local.get 7
        i32.const -100
        i32.mul
        local.get 6
        i32.add
        i32.const 65535
        i32.and
        i32.const 1
        i32.shl
        i32.const 1049758
        i32.add
        i32.load16_u align=1
        i32.store16 align=1
        local.get 3
        i32.const -4
        i32.add
        local.set 3
        local.get 0
        i64.const 99999999
        i64.gt_u
        local.set 5
        local.get 4
        local.set 0
        local.get 5
        br_if 0 (;@2;)
      end
    end
    block  ;; label = @1
      local.get 4
      i32.wrap_i64
      local.tee 5
      i32.const 99
      i32.le_u
      br_if 0 (;@1;)
      local.get 2
      i32.const 9
      i32.add
      local.get 3
      i32.const -2
      i32.add
      local.tee 3
      i32.add
      local.get 4
      i32.wrap_i64
      local.tee 6
      i32.const 65535
      i32.and
      i32.const 100
      i32.div_u
      local.tee 5
      i32.const -100
      i32.mul
      local.get 6
      i32.add
      i32.const 65535
      i32.and
      i32.const 1
      i32.shl
      i32.const 1049758
      i32.add
      i32.load16_u align=1
      i32.store16 align=1
    end
    block  ;; label = @1
      block  ;; label = @2
        local.get 5
        i32.const 10
        i32.lt_u
        br_if 0 (;@2;)
        local.get 2
        i32.const 9
        i32.add
        local.get 3
        i32.const -2
        i32.add
        local.tee 3
        i32.add
        local.get 5
        i32.const 1
        i32.shl
        i32.const 1049758
        i32.add
        i32.load16_u align=1
        i32.store16 align=1
        br 1 (;@1;)
      end
      local.get 2
      i32.const 9
      i32.add
      local.get 3
      i32.const -1
      i32.add
      local.tee 3
      i32.add
      local.get 5
      i32.const 48
      i32.add
      i32.store8
    end
    local.get 1
    i32.const 1053004
    i32.const 0
    local.get 2
    i32.const 9
    i32.add
    local.get 3
    i32.add
    i32.const 39
    local.get 3
    i32.sub
    call 37
    local.set 3
    local.get 2
    i32.const 48
    i32.add
    global.set 0
    local.get 3)
  (func (;37;) (type 11) (param i32 i32 i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32)
    local.get 0
    i32.load offset=28
    local.tee 5
    i32.const 1
    i32.and
    local.tee 6
    local.get 4
    i32.add
    local.set 7
    block  ;; label = @1
      block  ;; label = @2
        local.get 5
        i32.const 4
        i32.and
        br_if 0 (;@2;)
        i32.const 0
        local.set 1
        br 1 (;@1;)
      end
      block  ;; label = @2
        block  ;; label = @3
          local.get 2
          br_if 0 (;@3;)
          i32.const 0
          local.set 8
          br 1 (;@2;)
        end
        block  ;; label = @3
          local.get 2
          i32.const 3
          i32.and
          local.tee 9
          br_if 0 (;@3;)
          br 1 (;@2;)
        end
        i32.const 0
        local.set 8
        local.get 1
        local.set 10
        loop  ;; label = @3
          local.get 8
          local.get 10
          i32.load8_s
          i32.const -65
          i32.gt_s
          i32.add
          local.set 8
          local.get 10
          i32.const 1
          i32.add
          local.set 10
          local.get 9
          i32.const -1
          i32.add
          local.tee 9
          br_if 0 (;@3;)
        end
      end
      local.get 8
      local.get 7
      i32.add
      local.set 7
    end
    i32.const 43
    i32.const 1114112
    local.get 6
    select
    local.set 6
    block  ;; label = @1
      block  ;; label = @2
        local.get 0
        i32.load
        br_if 0 (;@2;)
        i32.const 1
        local.set 10
        local.get 0
        i32.const 20
        i32.add
        i32.load
        local.tee 8
        local.get 0
        i32.const 24
        i32.add
        i32.load
        local.tee 9
        local.get 6
        local.get 1
        local.get 2
        call 38
        br_if 1 (;@1;)
        local.get 8
        local.get 3
        local.get 4
        local.get 9
        i32.load offset=12
        call_indirect (type 1)
        return
      end
      block  ;; label = @2
        local.get 0
        i32.load offset=4
        local.tee 11
        local.get 7
        i32.gt_u
        br_if 0 (;@2;)
        i32.const 1
        local.set 10
        local.get 0
        i32.const 20
        i32.add
        i32.load
        local.tee 8
        local.get 0
        i32.const 24
        i32.add
        i32.load
        local.tee 9
        local.get 6
        local.get 1
        local.get 2
        call 38
        br_if 1 (;@1;)
        local.get 8
        local.get 3
        local.get 4
        local.get 9
        i32.load offset=12
        call_indirect (type 1)
        return
      end
      block  ;; label = @2
        local.get 5
        i32.const 8
        i32.and
        i32.eqz
        br_if 0 (;@2;)
        local.get 0
        i32.load offset=16
        local.set 5
        local.get 0
        i32.const 48
        i32.store offset=16
        local.get 0
        i32.load8_u offset=32
        local.set 12
        i32.const 1
        local.set 10
        local.get 0
        i32.const 1
        i32.store8 offset=32
        local.get 0
        i32.const 20
        i32.add
        i32.load
        local.tee 8
        local.get 0
        i32.const 24
        i32.add
        i32.load
        local.tee 9
        local.get 6
        local.get 1
        local.get 2
        call 38
        br_if 1 (;@1;)
        local.get 11
        local.get 7
        i32.sub
        i32.const 1
        i32.add
        local.set 10
        block  ;; label = @3
          loop  ;; label = @4
            local.get 10
            i32.const -1
            i32.add
            local.tee 10
            i32.eqz
            br_if 1 (;@3;)
            local.get 8
            i32.const 48
            local.get 9
            i32.load offset=16
            call_indirect (type 2)
            i32.eqz
            br_if 0 (;@4;)
          end
          i32.const 1
          return
        end
        i32.const 1
        local.set 10
        local.get 8
        local.get 3
        local.get 4
        local.get 9
        i32.load offset=12
        call_indirect (type 1)
        br_if 1 (;@1;)
        local.get 0
        local.get 12
        i32.store8 offset=32
        local.get 0
        local.get 5
        i32.store offset=16
        i32.const 0
        local.set 10
        br 1 (;@1;)
      end
      local.get 11
      local.get 7
      i32.sub
      local.tee 8
      local.set 5
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 0
            i32.load8_u offset=32
            local.tee 10
            br_table 2 (;@2;) 0 (;@4;) 1 (;@3;) 0 (;@4;) 2 (;@2;)
          end
          i32.const 0
          local.set 5
          local.get 8
          local.set 10
          br 1 (;@2;)
        end
        local.get 8
        i32.const 1
        i32.shr_u
        local.set 10
        local.get 8
        i32.const 1
        i32.add
        i32.const 1
        i32.shr_u
        local.set 5
      end
      local.get 10
      i32.const 1
      i32.add
      local.set 10
      local.get 0
      i32.const 24
      i32.add
      i32.load
      local.set 9
      local.get 0
      i32.const 20
      i32.add
      i32.load
      local.set 7
      local.get 0
      i32.load offset=16
      local.set 8
      block  ;; label = @2
        loop  ;; label = @3
          local.get 10
          i32.const -1
          i32.add
          local.tee 10
          i32.eqz
          br_if 1 (;@2;)
          local.get 7
          local.get 8
          local.get 9
          i32.load offset=16
          call_indirect (type 2)
          i32.eqz
          br_if 0 (;@3;)
        end
        i32.const 1
        return
      end
      i32.const 1
      local.set 10
      local.get 8
      i32.const 1114112
      i32.eq
      br_if 0 (;@1;)
      local.get 7
      local.get 9
      local.get 6
      local.get 1
      local.get 2
      call 38
      br_if 0 (;@1;)
      local.get 7
      local.get 3
      local.get 4
      local.get 9
      i32.load offset=12
      call_indirect (type 1)
      br_if 0 (;@1;)
      i32.const 0
      local.set 10
      loop  ;; label = @2
        block  ;; label = @3
          local.get 5
          local.get 10
          i32.ne
          br_if 0 (;@3;)
          local.get 5
          local.get 5
          i32.lt_u
          return
        end
        local.get 10
        i32.const 1
        i32.add
        local.set 10
        local.get 7
        local.get 8
        local.get 9
        i32.load offset=16
        call_indirect (type 2)
        i32.eqz
        br_if 0 (;@2;)
      end
      local.get 10
      i32.const -1
      i32.add
      local.get 5
      i32.lt_u
      return
    end
    local.get 10)
  (func (;38;) (type 11) (param i32 i32 i32 i32 i32) (result i32)
    (local i32)
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 2
          i32.const 1114112
          i32.eq
          br_if 0 (;@3;)
          i32.const 1
          local.set 5
          local.get 0
          local.get 2
          local.get 1
          i32.load offset=16
          call_indirect (type 2)
          br_if 1 (;@2;)
        end
        local.get 3
        br_if 1 (;@1;)
        i32.const 0
        local.set 5
      end
      local.get 5
      return
    end
    local.get 0
    local.get 3
    local.get 4
    local.get 1
    i32.load offset=12
    call_indirect (type 1))
  (func (;39;) (type 10) (param i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 48
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    local.get 0
    i32.store
    local.get 3
    local.get 1
    i32.store offset=4
    local.get 3
    i32.const 8
    i32.add
    i32.const 12
    i32.add
    i64.const 2
    i64.store align=4
    local.get 3
    i32.const 32
    i32.add
    i32.const 12
    i32.add
    i32.const 4
    i32.store
    local.get 3
    i32.const 2
    i32.store offset=12
    local.get 3
    i32.const 1050072
    i32.store offset=8
    local.get 3
    i32.const 4
    i32.store offset=36
    local.get 3
    local.get 3
    i32.const 32
    i32.add
    i32.store offset=16
    local.get 3
    local.get 3
    i32.const 4
    i32.add
    i32.store offset=40
    local.get 3
    local.get 3
    i32.store offset=32
    local.get 3
    i32.const 8
    i32.add
    local.get 2
    call 24
    unreachable)
  (func (;40;) (type 12) (param i32 i32)
    (local i32)
    global.get 0
    i32.const 48
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    local.get 0
    i32.store
    local.get 2
    local.get 1
    i32.store offset=4
    local.get 2
    i32.const 8
    i32.add
    i32.const 12
    i32.add
    i64.const 2
    i64.store align=4
    local.get 2
    i32.const 32
    i32.add
    i32.const 12
    i32.add
    i32.const 4
    i32.store
    local.get 2
    i32.const 2
    i32.store offset=12
    local.get 2
    i32.const 1050104
    i32.store offset=8
    local.get 2
    i32.const 4
    i32.store offset=36
    local.get 2
    local.get 2
    i32.const 32
    i32.add
    i32.store offset=16
    local.get 2
    local.get 2
    i32.const 4
    i32.add
    i32.store offset=40
    local.get 2
    local.get 2
    i32.store offset=32
    local.get 2
    i32.const 8
    i32.add
    i32.const 1050496
    call 24
    unreachable)
  (func (;41;) (type 1) (param i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 0
          i32.load
          local.tee 3
          local.get 0
          i32.load offset=8
          local.tee 4
          i32.or
          i32.eqz
          br_if 0 (;@3;)
          block  ;; label = @4
            local.get 4
            i32.eqz
            br_if 0 (;@4;)
            local.get 1
            local.get 2
            i32.add
            local.set 5
            local.get 0
            i32.const 12
            i32.add
            i32.load
            i32.const 1
            i32.add
            local.set 6
            i32.const 0
            local.set 7
            local.get 1
            local.set 8
            block  ;; label = @5
              loop  ;; label = @6
                local.get 8
                local.set 4
                local.get 6
                i32.const -1
                i32.add
                local.tee 6
                i32.eqz
                br_if 1 (;@5;)
                local.get 4
                local.get 5
                i32.eq
                br_if 2 (;@4;)
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 4
                    i32.load8_s
                    local.tee 9
                    i32.const -1
                    i32.le_s
                    br_if 0 (;@8;)
                    local.get 4
                    i32.const 1
                    i32.add
                    local.set 8
                    local.get 9
                    i32.const 255
                    i32.and
                    local.set 9
                    br 1 (;@7;)
                  end
                  local.get 4
                  i32.load8_u offset=1
                  i32.const 63
                  i32.and
                  local.set 10
                  local.get 9
                  i32.const 31
                  i32.and
                  local.set 8
                  block  ;; label = @8
                    local.get 9
                    i32.const -33
                    i32.gt_u
                    br_if 0 (;@8;)
                    local.get 8
                    i32.const 6
                    i32.shl
                    local.get 10
                    i32.or
                    local.set 9
                    local.get 4
                    i32.const 2
                    i32.add
                    local.set 8
                    br 1 (;@7;)
                  end
                  local.get 10
                  i32.const 6
                  i32.shl
                  local.get 4
                  i32.load8_u offset=2
                  i32.const 63
                  i32.and
                  i32.or
                  local.set 10
                  block  ;; label = @8
                    local.get 9
                    i32.const -16
                    i32.ge_u
                    br_if 0 (;@8;)
                    local.get 10
                    local.get 8
                    i32.const 12
                    i32.shl
                    i32.or
                    local.set 9
                    local.get 4
                    i32.const 3
                    i32.add
                    local.set 8
                    br 1 (;@7;)
                  end
                  local.get 10
                  i32.const 6
                  i32.shl
                  local.get 4
                  i32.load8_u offset=3
                  i32.const 63
                  i32.and
                  i32.or
                  local.get 8
                  i32.const 18
                  i32.shl
                  i32.const 1835008
                  i32.and
                  i32.or
                  local.tee 9
                  i32.const 1114112
                  i32.eq
                  br_if 3 (;@4;)
                  local.get 4
                  i32.const 4
                  i32.add
                  local.set 8
                end
                local.get 7
                local.get 4
                i32.sub
                local.get 8
                i32.add
                local.set 7
                local.get 9
                i32.const 1114112
                i32.ne
                br_if 0 (;@6;)
                br 2 (;@4;)
              end
            end
            local.get 4
            local.get 5
            i32.eq
            br_if 0 (;@4;)
            block  ;; label = @5
              local.get 4
              i32.load8_s
              local.tee 8
              i32.const -1
              i32.gt_s
              br_if 0 (;@5;)
              local.get 8
              i32.const -32
              i32.lt_u
              br_if 0 (;@5;)
              local.get 8
              i32.const -16
              i32.lt_u
              br_if 0 (;@5;)
              local.get 4
              i32.load8_u offset=2
              i32.const 63
              i32.and
              i32.const 6
              i32.shl
              local.get 4
              i32.load8_u offset=1
              i32.const 63
              i32.and
              i32.const 12
              i32.shl
              i32.or
              local.get 4
              i32.load8_u offset=3
              i32.const 63
              i32.and
              i32.or
              local.get 8
              i32.const 255
              i32.and
              i32.const 18
              i32.shl
              i32.const 1835008
              i32.and
              i32.or
              i32.const 1114112
              i32.eq
              br_if 1 (;@4;)
            end
            block  ;; label = @5
              block  ;; label = @6
                local.get 7
                i32.eqz
                br_if 0 (;@6;)
                block  ;; label = @7
                  local.get 7
                  local.get 2
                  i32.lt_u
                  br_if 0 (;@7;)
                  i32.const 0
                  local.set 4
                  local.get 7
                  local.get 2
                  i32.eq
                  br_if 1 (;@6;)
                  br 2 (;@5;)
                end
                i32.const 0
                local.set 4
                local.get 1
                local.get 7
                i32.add
                i32.load8_s
                i32.const -64
                i32.lt_s
                br_if 1 (;@5;)
              end
              local.get 1
              local.set 4
            end
            local.get 7
            local.get 2
            local.get 4
            select
            local.set 2
            local.get 4
            local.get 1
            local.get 4
            select
            local.set 1
          end
          block  ;; label = @4
            local.get 3
            br_if 0 (;@4;)
            local.get 0
            i32.load offset=20
            local.get 1
            local.get 2
            local.get 0
            i32.const 24
            i32.add
            i32.load
            i32.load offset=12
            call_indirect (type 1)
            return
          end
          local.get 0
          i32.load offset=4
          local.set 11
          block  ;; label = @4
            local.get 2
            i32.const 16
            i32.lt_u
            br_if 0 (;@4;)
            local.get 2
            local.get 1
            i32.const 3
            i32.add
            i32.const -4
            i32.and
            local.tee 9
            local.get 1
            i32.sub
            local.tee 8
            i32.sub
            local.tee 3
            i32.const 3
            i32.and
            local.set 5
            i32.const 0
            local.set 10
            i32.const 0
            local.set 4
            block  ;; label = @5
              local.get 9
              local.get 1
              i32.eq
              br_if 0 (;@5;)
              local.get 8
              i32.const 3
              i32.and
              local.set 7
              i32.const 0
              local.set 4
              block  ;; label = @6
                local.get 9
                local.get 1
                i32.const -1
                i32.xor
                i32.add
                i32.const 3
                i32.lt_u
                br_if 0 (;@6;)
                i32.const 0
                local.set 6
                loop  ;; label = @7
                  local.get 4
                  local.get 1
                  local.get 6
                  i32.add
                  local.tee 8
                  i32.load8_s
                  i32.const -65
                  i32.gt_s
                  i32.add
                  local.get 8
                  i32.const 1
                  i32.add
                  i32.load8_s
                  i32.const -65
                  i32.gt_s
                  i32.add
                  local.get 8
                  i32.const 2
                  i32.add
                  i32.load8_s
                  i32.const -65
                  i32.gt_s
                  i32.add
                  local.get 8
                  i32.const 3
                  i32.add
                  i32.load8_s
                  i32.const -65
                  i32.gt_s
                  i32.add
                  local.set 4
                  local.get 6
                  i32.const 4
                  i32.add
                  local.tee 6
                  br_if 0 (;@7;)
                end
              end
              local.get 7
              i32.eqz
              br_if 0 (;@5;)
              local.get 1
              local.set 8
              loop  ;; label = @6
                local.get 4
                local.get 8
                i32.load8_s
                i32.const -65
                i32.gt_s
                i32.add
                local.set 4
                local.get 8
                i32.const 1
                i32.add
                local.set 8
                local.get 7
                i32.const -1
                i32.add
                local.tee 7
                br_if 0 (;@6;)
              end
            end
            block  ;; label = @5
              local.get 5
              i32.eqz
              br_if 0 (;@5;)
              local.get 9
              local.get 3
              i32.const -4
              i32.and
              i32.add
              local.tee 8
              i32.load8_s
              i32.const -65
              i32.gt_s
              local.set 10
              local.get 5
              i32.const 1
              i32.eq
              br_if 0 (;@5;)
              local.get 10
              local.get 8
              i32.load8_s offset=1
              i32.const -65
              i32.gt_s
              i32.add
              local.set 10
              local.get 5
              i32.const 2
              i32.eq
              br_if 0 (;@5;)
              local.get 10
              local.get 8
              i32.load8_s offset=2
              i32.const -65
              i32.gt_s
              i32.add
              local.set 10
            end
            local.get 3
            i32.const 2
            i32.shr_u
            local.set 5
            local.get 10
            local.get 4
            i32.add
            local.set 7
            loop  ;; label = @5
              local.get 9
              local.set 3
              local.get 5
              i32.eqz
              br_if 4 (;@1;)
              local.get 5
              i32.const 192
              local.get 5
              i32.const 192
              i32.lt_u
              select
              local.tee 10
              i32.const 3
              i32.and
              local.set 12
              local.get 10
              i32.const 2
              i32.shl
              local.set 13
              block  ;; label = @6
                block  ;; label = @7
                  local.get 10
                  i32.const 252
                  i32.and
                  local.tee 14
                  br_if 0 (;@7;)
                  i32.const 0
                  local.set 8
                  br 1 (;@6;)
                end
                local.get 3
                local.get 14
                i32.const 2
                i32.shl
                i32.add
                local.set 6
                i32.const 0
                local.set 8
                local.get 3
                local.set 4
                loop  ;; label = @7
                  local.get 4
                  i32.eqz
                  br_if 1 (;@6;)
                  local.get 4
                  i32.const 12
                  i32.add
                  i32.load
                  local.tee 9
                  i32.const -1
                  i32.xor
                  i32.const 7
                  i32.shr_u
                  local.get 9
                  i32.const 6
                  i32.shr_u
                  i32.or
                  i32.const 16843009
                  i32.and
                  local.get 4
                  i32.const 8
                  i32.add
                  i32.load
                  local.tee 9
                  i32.const -1
                  i32.xor
                  i32.const 7
                  i32.shr_u
                  local.get 9
                  i32.const 6
                  i32.shr_u
                  i32.or
                  i32.const 16843009
                  i32.and
                  local.get 4
                  i32.const 4
                  i32.add
                  i32.load
                  local.tee 9
                  i32.const -1
                  i32.xor
                  i32.const 7
                  i32.shr_u
                  local.get 9
                  i32.const 6
                  i32.shr_u
                  i32.or
                  i32.const 16843009
                  i32.and
                  local.get 4
                  i32.load
                  local.tee 9
                  i32.const -1
                  i32.xor
                  i32.const 7
                  i32.shr_u
                  local.get 9
                  i32.const 6
                  i32.shr_u
                  i32.or
                  i32.const 16843009
                  i32.and
                  local.get 8
                  i32.add
                  i32.add
                  i32.add
                  i32.add
                  local.set 8
                  local.get 4
                  i32.const 16
                  i32.add
                  local.tee 4
                  local.get 6
                  i32.ne
                  br_if 0 (;@7;)
                end
              end
              local.get 5
              local.get 10
              i32.sub
              local.set 5
              local.get 3
              local.get 13
              i32.add
              local.set 9
              local.get 8
              i32.const 8
              i32.shr_u
              i32.const 16711935
              i32.and
              local.get 8
              i32.const 16711935
              i32.and
              i32.add
              i32.const 65537
              i32.mul
              i32.const 16
              i32.shr_u
              local.get 7
              i32.add
              local.set 7
              local.get 12
              i32.eqz
              br_if 0 (;@5;)
            end
            block  ;; label = @5
              local.get 3
              br_if 0 (;@5;)
              i32.const 0
              local.set 4
              br 3 (;@2;)
            end
            local.get 3
            local.get 14
            i32.const 2
            i32.shl
            i32.add
            local.tee 8
            i32.load
            local.tee 4
            i32.const -1
            i32.xor
            i32.const 7
            i32.shr_u
            local.get 4
            i32.const 6
            i32.shr_u
            i32.or
            i32.const 16843009
            i32.and
            local.set 4
            local.get 12
            i32.const 1
            i32.eq
            br_if 2 (;@2;)
            local.get 8
            i32.load offset=4
            local.tee 9
            i32.const -1
            i32.xor
            i32.const 7
            i32.shr_u
            local.get 9
            i32.const 6
            i32.shr_u
            i32.or
            i32.const 16843009
            i32.and
            local.get 4
            i32.add
            local.set 4
            local.get 12
            i32.const 2
            i32.eq
            br_if 2 (;@2;)
            local.get 8
            i32.load offset=8
            local.tee 8
            i32.const -1
            i32.xor
            i32.const 7
            i32.shr_u
            local.get 8
            i32.const 6
            i32.shr_u
            i32.or
            i32.const 16843009
            i32.and
            local.get 4
            i32.add
            local.set 4
            br 2 (;@2;)
          end
          block  ;; label = @4
            local.get 2
            br_if 0 (;@4;)
            i32.const 0
            local.set 7
            br 3 (;@1;)
          end
          local.get 2
          i32.const 3
          i32.and
          local.set 8
          block  ;; label = @4
            block  ;; label = @5
              local.get 2
              i32.const 4
              i32.ge_u
              br_if 0 (;@5;)
              i32.const 0
              local.set 7
              i32.const 0
              local.set 6
              br 1 (;@4;)
            end
            i32.const 0
            local.set 7
            local.get 1
            local.set 4
            local.get 2
            i32.const -4
            i32.and
            local.tee 6
            local.set 9
            loop  ;; label = @5
              local.get 7
              local.get 4
              i32.load8_s
              i32.const -65
              i32.gt_s
              i32.add
              local.get 4
              i32.const 1
              i32.add
              i32.load8_s
              i32.const -65
              i32.gt_s
              i32.add
              local.get 4
              i32.const 2
              i32.add
              i32.load8_s
              i32.const -65
              i32.gt_s
              i32.add
              local.get 4
              i32.const 3
              i32.add
              i32.load8_s
              i32.const -65
              i32.gt_s
              i32.add
              local.set 7
              local.get 4
              i32.const 4
              i32.add
              local.set 4
              local.get 9
              i32.const -4
              i32.add
              local.tee 9
              br_if 0 (;@5;)
            end
          end
          local.get 8
          i32.eqz
          br_if 2 (;@1;)
          local.get 1
          local.get 6
          i32.add
          local.set 4
          loop  ;; label = @4
            local.get 7
            local.get 4
            i32.load8_s
            i32.const -65
            i32.gt_s
            i32.add
            local.set 7
            local.get 4
            i32.const 1
            i32.add
            local.set 4
            local.get 8
            i32.const -1
            i32.add
            local.tee 8
            br_if 0 (;@4;)
            br 3 (;@1;)
          end
        end
        local.get 0
        i32.load offset=20
        local.get 1
        local.get 2
        local.get 0
        i32.const 24
        i32.add
        i32.load
        i32.load offset=12
        call_indirect (type 1)
        return
      end
      local.get 4
      i32.const 8
      i32.shr_u
      i32.const 459007
      i32.and
      local.get 4
      i32.const 16711935
      i32.and
      i32.add
      i32.const 65537
      i32.mul
      i32.const 16
      i32.shr_u
      local.get 7
      i32.add
      local.set 7
    end
    block  ;; label = @1
      local.get 11
      local.get 7
      i32.le_u
      br_if 0 (;@1;)
      i32.const 0
      local.set 4
      local.get 11
      local.get 7
      i32.sub
      local.tee 8
      local.set 7
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 0
            i32.load8_u offset=32
            br_table 2 (;@2;) 0 (;@4;) 1 (;@3;) 2 (;@2;) 2 (;@2;)
          end
          i32.const 0
          local.set 7
          local.get 8
          local.set 4
          br 1 (;@2;)
        end
        local.get 8
        i32.const 1
        i32.shr_u
        local.set 4
        local.get 8
        i32.const 1
        i32.add
        i32.const 1
        i32.shr_u
        local.set 7
      end
      local.get 4
      i32.const 1
      i32.add
      local.set 4
      local.get 0
      i32.const 24
      i32.add
      i32.load
      local.set 9
      local.get 0
      i32.const 20
      i32.add
      i32.load
      local.set 6
      local.get 0
      i32.load offset=16
      local.set 8
      block  ;; label = @2
        loop  ;; label = @3
          local.get 4
          i32.const -1
          i32.add
          local.tee 4
          i32.eqz
          br_if 1 (;@2;)
          local.get 6
          local.get 8
          local.get 9
          i32.load offset=16
          call_indirect (type 2)
          i32.eqz
          br_if 0 (;@3;)
        end
        i32.const 1
        return
      end
      i32.const 1
      local.set 4
      block  ;; label = @2
        local.get 8
        i32.const 1114112
        i32.eq
        br_if 0 (;@2;)
        local.get 6
        local.get 1
        local.get 2
        local.get 9
        i32.load offset=12
        call_indirect (type 1)
        br_if 0 (;@2;)
        i32.const 0
        local.set 4
        block  ;; label = @3
          loop  ;; label = @4
            block  ;; label = @5
              local.get 7
              local.get 4
              i32.ne
              br_if 0 (;@5;)
              local.get 7
              local.set 4
              br 2 (;@3;)
            end
            local.get 4
            i32.const 1
            i32.add
            local.set 4
            local.get 6
            local.get 8
            local.get 9
            i32.load offset=16
            call_indirect (type 2)
            i32.eqz
            br_if 0 (;@4;)
          end
          local.get 4
          i32.const -1
          i32.add
          local.set 4
        end
        local.get 4
        local.get 7
        i32.lt_u
        local.set 4
      end
      local.get 4
      return
    end
    local.get 0
    i32.load offset=20
    local.get 1
    local.get 2
    local.get 0
    i32.const 24
    i32.add
    i32.load
    i32.load offset=12
    call_indirect (type 1))
  (func (;42;) (type 2) (param i32 i32) (result i32)
    local.get 0
    i32.load
    local.get 1
    local.get 0
    i32.load offset=4
    i32.load offset=12
    call_indirect (type 2))
  (func (;43;) (type 2) (param i32 i32) (result i32)
    local.get 1
    local.get 0
    i32.load
    local.get 0
    i32.load offset=4
    call 41)
  (func (;44;) (type 2) (param i32 i32) (result i32)
    local.get 1
    i32.load offset=20
    local.get 1
    i32.const 24
    i32.add
    i32.load
    local.get 0
    call 45)
  (func (;45;) (type 1) (param i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    global.get 0
    i32.const 48
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    i32.const 32
    i32.add
    local.get 1
    i32.store
    local.get 3
    i32.const 3
    i32.store8 offset=40
    local.get 3
    i32.const 32
    i32.store offset=24
    i32.const 0
    local.set 4
    local.get 3
    i32.const 0
    i32.store offset=36
    local.get 3
    local.get 0
    i32.store offset=28
    local.get 3
    i32.const 0
    i32.store offset=16
    local.get 3
    i32.const 0
    i32.store offset=8
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 2
            i32.load offset=16
            local.tee 5
            br_if 0 (;@4;)
            local.get 2
            i32.const 12
            i32.add
            i32.load
            local.tee 0
            i32.eqz
            br_if 1 (;@3;)
            local.get 2
            i32.load offset=8
            local.set 1
            local.get 0
            i32.const 3
            i32.shl
            local.set 6
            local.get 0
            i32.const -1
            i32.add
            i32.const 536870911
            i32.and
            i32.const 1
            i32.add
            local.set 4
            local.get 2
            i32.load
            local.set 0
            loop  ;; label = @5
              block  ;; label = @6
                local.get 0
                i32.const 4
                i32.add
                i32.load
                local.tee 7
                i32.eqz
                br_if 0 (;@6;)
                local.get 3
                i32.load offset=28
                local.get 0
                i32.load
                local.get 7
                local.get 3
                i32.load offset=32
                i32.load offset=12
                call_indirect (type 1)
                br_if 4 (;@2;)
              end
              local.get 1
              i32.load
              local.get 3
              i32.const 8
              i32.add
              local.get 1
              i32.const 4
              i32.add
              i32.load
              call_indirect (type 2)
              br_if 3 (;@2;)
              local.get 1
              i32.const 8
              i32.add
              local.set 1
              local.get 0
              i32.const 8
              i32.add
              local.set 0
              local.get 6
              i32.const -8
              i32.add
              local.tee 6
              br_if 0 (;@5;)
              br 2 (;@3;)
            end
          end
          local.get 2
          i32.const 20
          i32.add
          i32.load
          local.tee 1
          i32.eqz
          br_if 0 (;@3;)
          local.get 1
          i32.const 5
          i32.shl
          local.set 8
          local.get 1
          i32.const -1
          i32.add
          i32.const 134217727
          i32.and
          i32.const 1
          i32.add
          local.set 4
          local.get 2
          i32.load
          local.set 0
          i32.const 0
          local.set 6
          loop  ;; label = @4
            block  ;; label = @5
              local.get 0
              i32.const 4
              i32.add
              i32.load
              local.tee 1
              i32.eqz
              br_if 0 (;@5;)
              local.get 3
              i32.load offset=28
              local.get 0
              i32.load
              local.get 1
              local.get 3
              i32.load offset=32
              i32.load offset=12
              call_indirect (type 1)
              br_if 3 (;@2;)
            end
            local.get 3
            local.get 5
            local.get 6
            i32.add
            local.tee 1
            i32.const 16
            i32.add
            i32.load
            i32.store offset=24
            local.get 3
            local.get 1
            i32.const 28
            i32.add
            i32.load8_u
            i32.store8 offset=40
            local.get 3
            local.get 1
            i32.const 24
            i32.add
            i32.load
            i32.store offset=36
            local.get 1
            i32.const 12
            i32.add
            i32.load
            local.set 9
            local.get 2
            i32.load offset=8
            local.set 10
            i32.const 0
            local.set 11
            i32.const 0
            local.set 7
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  local.get 1
                  i32.const 8
                  i32.add
                  i32.load
                  br_table 1 (;@6;) 0 (;@7;) 2 (;@5;) 1 (;@6;)
                end
                local.get 9
                i32.const 3
                i32.shl
                local.set 12
                i32.const 0
                local.set 7
                local.get 10
                local.get 12
                i32.add
                local.tee 12
                i32.load offset=4
                i32.const 5
                i32.ne
                br_if 1 (;@5;)
                local.get 12
                i32.load
                i32.load
                local.set 9
              end
              i32.const 1
              local.set 7
            end
            local.get 3
            local.get 9
            i32.store offset=12
            local.get 3
            local.get 7
            i32.store offset=8
            local.get 1
            i32.const 4
            i32.add
            i32.load
            local.set 7
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  local.get 1
                  i32.load
                  br_table 1 (;@6;) 0 (;@7;) 2 (;@5;) 1 (;@6;)
                end
                local.get 7
                i32.const 3
                i32.shl
                local.set 9
                local.get 10
                local.get 9
                i32.add
                local.tee 9
                i32.load offset=4
                i32.const 5
                i32.ne
                br_if 1 (;@5;)
                local.get 9
                i32.load
                i32.load
                local.set 7
              end
              i32.const 1
              local.set 11
            end
            local.get 3
            local.get 7
            i32.store offset=20
            local.get 3
            local.get 11
            i32.store offset=16
            local.get 10
            local.get 1
            i32.const 20
            i32.add
            i32.load
            i32.const 3
            i32.shl
            i32.add
            local.tee 1
            i32.load
            local.get 3
            i32.const 8
            i32.add
            local.get 1
            i32.load offset=4
            call_indirect (type 2)
            br_if 2 (;@2;)
            local.get 0
            i32.const 8
            i32.add
            local.set 0
            local.get 8
            local.get 6
            i32.const 32
            i32.add
            local.tee 6
            i32.ne
            br_if 0 (;@4;)
          end
        end
        block  ;; label = @3
          local.get 4
          local.get 2
          i32.load offset=4
          i32.ge_u
          br_if 0 (;@3;)
          local.get 3
          i32.load offset=28
          local.get 2
          i32.load
          local.get 4
          i32.const 3
          i32.shl
          i32.add
          local.tee 1
          i32.load
          local.get 1
          i32.load offset=4
          local.get 3
          i32.load offset=32
          i32.load offset=12
          call_indirect (type 1)
          br_if 1 (;@2;)
        end
        i32.const 0
        local.set 1
        br 1 (;@1;)
      end
      i32.const 1
      local.set 1
    end
    local.get 3
    i32.const 48
    i32.add
    global.set 0
    local.get 1)
  (func (;46;) (type 0) (param i32))
  (func (;47;) (type 10) (param i32 i32 i32)
    (local i32)
    global.get 0
    i32.const 48
    i32.sub
    local.tee 3
    global.set 0
    local.get 3
    local.get 0
    i32.store
    local.get 3
    local.get 1
    i32.store offset=4
    local.get 3
    i32.const 8
    i32.add
    i32.const 12
    i32.add
    i64.const 2
    i64.store align=4
    local.get 3
    i32.const 32
    i32.add
    i32.const 12
    i32.add
    i32.const 4
    i32.store
    local.get 3
    i32.const 2
    i32.store offset=12
    local.get 3
    i32.const 1050156
    i32.store offset=8
    local.get 3
    i32.const 4
    i32.store offset=36
    local.get 3
    local.get 3
    i32.const 32
    i32.add
    i32.store offset=16
    local.get 3
    local.get 3
    i32.const 4
    i32.add
    i32.store offset=40
    local.get 3
    local.get 3
    i32.store offset=32
    local.get 3
    i32.const 8
    i32.add
    local.get 2
    call 24
    unreachable)
  (func (;48;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32)
    global.get 0
    i32.const 128
    i32.sub
    local.tee 2
    global.set 0
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.load offset=28
            local.tee 3
            i32.const 16
            i32.and
            br_if 0 (;@4;)
            local.get 3
            i32.const 32
            i32.and
            br_if 1 (;@3;)
            local.get 0
            i64.extend_i32_u
            local.get 1
            call 36
            local.set 0
            br 3 (;@1;)
          end
          i32.const 0
          local.set 3
          loop  ;; label = @4
            local.get 2
            local.get 3
            i32.add
            i32.const 127
            i32.add
            i32.const 48
            i32.const 87
            local.get 0
            i32.const 15
            i32.and
            local.tee 4
            i32.const 10
            i32.lt_u
            select
            local.get 4
            i32.add
            i32.store8
            local.get 3
            i32.const -1
            i32.add
            local.set 3
            local.get 0
            i32.const 16
            i32.lt_u
            local.set 4
            local.get 0
            i32.const 4
            i32.shr_u
            local.set 0
            local.get 4
            i32.eqz
            br_if 0 (;@4;)
            br 2 (;@2;)
          end
        end
        i32.const 0
        local.set 3
        loop  ;; label = @3
          local.get 2
          local.get 3
          i32.add
          i32.const 127
          i32.add
          i32.const 48
          i32.const 55
          local.get 0
          i32.const 15
          i32.and
          local.tee 4
          i32.const 10
          i32.lt_u
          select
          local.get 4
          i32.add
          i32.store8
          local.get 3
          i32.const -1
          i32.add
          local.set 3
          local.get 0
          i32.const 16
          i32.lt_u
          local.set 4
          local.get 0
          i32.const 4
          i32.shr_u
          local.set 0
          local.get 4
          i32.eqz
          br_if 0 (;@3;)
        end
        block  ;; label = @3
          local.get 3
          i32.const 128
          i32.add
          local.tee 0
          i32.const 129
          i32.lt_u
          br_if 0 (;@3;)
          local.get 0
          i32.const 128
          i32.const 1049740
          call 39
          unreachable
        end
        local.get 1
        i32.const 1049756
        i32.const 2
        local.get 2
        local.get 3
        i32.add
        i32.const 128
        i32.add
        i32.const 0
        local.get 3
        i32.sub
        call 37
        local.set 0
        br 1 (;@1;)
      end
      block  ;; label = @2
        local.get 3
        i32.const 128
        i32.add
        local.tee 0
        i32.const 129
        i32.lt_u
        br_if 0 (;@2;)
        local.get 0
        i32.const 128
        i32.const 1049740
        call 39
        unreachable
      end
      local.get 1
      i32.const 1049756
      i32.const 2
      local.get 2
      local.get 3
      i32.add
      i32.const 128
      i32.add
      i32.const 0
      local.get 3
      i32.sub
      call 37
      local.set 0
    end
    local.get 2
    i32.const 128
    i32.add
    global.set 0
    local.get 0)
  (func (;49;) (type 1) (param i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32)
    local.get 0
    i32.load offset=4
    local.set 3
    local.get 0
    i32.load
    local.set 4
    local.get 0
    i32.load offset=8
    local.set 5
    i32.const 0
    local.set 6
    i32.const 0
    local.set 7
    i32.const 0
    local.set 8
    i32.const 0
    local.set 9
    block  ;; label = @1
      loop  ;; label = @2
        local.get 9
        i32.const 255
        i32.and
        br_if 1 (;@1;)
        block  ;; label = @3
          block  ;; label = @4
            local.get 8
            local.get 2
            i32.gt_u
            br_if 0 (;@4;)
            loop  ;; label = @5
              local.get 1
              local.get 8
              i32.add
              local.set 10
              block  ;; label = @6
                block  ;; label = @7
                  local.get 2
                  local.get 8
                  i32.sub
                  local.tee 11
                  i32.const 8
                  i32.lt_u
                  br_if 0 (;@7;)
                  block  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 10
                        i32.const 3
                        i32.add
                        i32.const -4
                        i32.and
                        local.tee 0
                        local.get 10
                        i32.eq
                        br_if 0 (;@10;)
                        local.get 0
                        local.get 10
                        i32.sub
                        local.tee 12
                        i32.eqz
                        br_if 0 (;@10;)
                        i32.const 0
                        local.set 0
                        loop  ;; label = @11
                          local.get 10
                          local.get 0
                          i32.add
                          i32.load8_u
                          i32.const 10
                          i32.eq
                          br_if 5 (;@6;)
                          local.get 12
                          local.get 0
                          i32.const 1
                          i32.add
                          local.tee 0
                          i32.ne
                          br_if 0 (;@11;)
                        end
                        local.get 12
                        local.get 11
                        i32.const -8
                        i32.add
                        local.tee 13
                        i32.le_u
                        br_if 1 (;@9;)
                        br 2 (;@8;)
                      end
                      local.get 11
                      i32.const -8
                      i32.add
                      local.set 13
                      i32.const 0
                      local.set 12
                    end
                    loop  ;; label = @9
                      local.get 10
                      local.get 12
                      i32.add
                      local.tee 9
                      i32.load
                      local.tee 0
                      i32.const -1
                      i32.xor
                      local.get 0
                      i32.const 168430090
                      i32.xor
                      i32.const -16843009
                      i32.add
                      i32.and
                      i32.const -2139062144
                      i32.and
                      br_if 1 (;@8;)
                      local.get 9
                      i32.const 4
                      i32.add
                      i32.load
                      local.tee 0
                      i32.const -1
                      i32.xor
                      local.get 0
                      i32.const 168430090
                      i32.xor
                      i32.const -16843009
                      i32.add
                      i32.and
                      i32.const -2139062144
                      i32.and
                      br_if 1 (;@8;)
                      local.get 12
                      i32.const 8
                      i32.add
                      local.tee 12
                      local.get 13
                      i32.le_u
                      br_if 0 (;@9;)
                    end
                  end
                  block  ;; label = @8
                    local.get 11
                    local.get 12
                    i32.ne
                    br_if 0 (;@8;)
                    local.get 2
                    local.set 8
                    br 4 (;@4;)
                  end
                  loop  ;; label = @8
                    block  ;; label = @9
                      local.get 10
                      local.get 12
                      i32.add
                      i32.load8_u
                      i32.const 10
                      i32.ne
                      br_if 0 (;@9;)
                      local.get 12
                      local.set 0
                      br 3 (;@6;)
                    end
                    local.get 11
                    local.get 12
                    i32.const 1
                    i32.add
                    local.tee 12
                    i32.ne
                    br_if 0 (;@8;)
                  end
                  local.get 2
                  local.set 8
                  br 3 (;@4;)
                end
                block  ;; label = @7
                  local.get 8
                  local.get 2
                  i32.ne
                  br_if 0 (;@7;)
                  local.get 2
                  local.set 8
                  br 3 (;@4;)
                end
                i32.const 0
                local.set 0
                loop  ;; label = @7
                  local.get 10
                  local.get 0
                  i32.add
                  i32.load8_u
                  i32.const 10
                  i32.eq
                  br_if 1 (;@6;)
                  local.get 11
                  local.get 0
                  i32.const 1
                  i32.add
                  local.tee 0
                  i32.ne
                  br_if 0 (;@7;)
                end
                local.get 2
                local.set 8
                br 2 (;@4;)
              end
              local.get 8
              local.get 0
              i32.add
              local.tee 0
              i32.const 1
              i32.add
              local.set 8
              block  ;; label = @6
                local.get 0
                local.get 2
                i32.ge_u
                br_if 0 (;@6;)
                local.get 1
                local.get 0
                i32.add
                i32.load8_u
                i32.const 10
                i32.ne
                br_if 0 (;@6;)
                i32.const 0
                local.set 9
                local.get 8
                local.set 13
                local.get 8
                local.set 0
                br 3 (;@3;)
              end
              local.get 8
              local.get 2
              i32.le_u
              br_if 0 (;@5;)
            end
          end
          i32.const 1
          local.set 9
          local.get 7
          local.set 13
          local.get 2
          local.set 0
          local.get 7
          local.get 2
          i32.eq
          br_if 2 (;@1;)
        end
        block  ;; label = @3
          block  ;; label = @4
            local.get 5
            i32.load8_u
            i32.eqz
            br_if 0 (;@4;)
            local.get 4
            i32.const 1049696
            i32.const 4
            local.get 3
            i32.load offset=12
            call_indirect (type 1)
            br_if 1 (;@3;)
          end
          local.get 1
          local.get 7
          i32.add
          local.set 12
          local.get 0
          local.get 7
          i32.sub
          local.set 10
          i32.const 0
          local.set 11
          block  ;; label = @4
            local.get 0
            local.get 7
            i32.eq
            br_if 0 (;@4;)
            local.get 10
            local.get 12
            i32.add
            i32.const -1
            i32.add
            i32.load8_u
            i32.const 10
            i32.eq
            local.set 11
          end
          local.get 5
          local.get 11
          i32.store8
          local.get 13
          local.set 7
          local.get 4
          local.get 12
          local.get 10
          local.get 3
          i32.load offset=12
          call_indirect (type 1)
          i32.eqz
          br_if 1 (;@2;)
        end
      end
      i32.const 1
      local.set 6
    end
    local.get 6)
  (func (;50;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    i32.const 0
    i32.store offset=12
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.const 128
            i32.lt_u
            br_if 0 (;@4;)
            local.get 1
            i32.const 2048
            i32.lt_u
            br_if 1 (;@3;)
            local.get 1
            i32.const 65536
            i32.ge_u
            br_if 2 (;@2;)
            local.get 2
            local.get 1
            i32.const 63
            i32.and
            i32.const 128
            i32.or
            i32.store8 offset=14
            local.get 2
            local.get 1
            i32.const 12
            i32.shr_u
            i32.const 224
            i32.or
            i32.store8 offset=12
            local.get 2
            local.get 1
            i32.const 6
            i32.shr_u
            i32.const 63
            i32.and
            i32.const 128
            i32.or
            i32.store8 offset=13
            i32.const 3
            local.set 1
            br 3 (;@1;)
          end
          local.get 2
          local.get 1
          i32.store8 offset=12
          i32.const 1
          local.set 1
          br 2 (;@1;)
        end
        local.get 2
        local.get 1
        i32.const 63
        i32.and
        i32.const 128
        i32.or
        i32.store8 offset=13
        local.get 2
        local.get 1
        i32.const 6
        i32.shr_u
        i32.const 192
        i32.or
        i32.store8 offset=12
        i32.const 2
        local.set 1
        br 1 (;@1;)
      end
      local.get 2
      local.get 1
      i32.const 63
      i32.and
      i32.const 128
      i32.or
      i32.store8 offset=15
      local.get 2
      local.get 1
      i32.const 6
      i32.shr_u
      i32.const 63
      i32.and
      i32.const 128
      i32.or
      i32.store8 offset=14
      local.get 2
      local.get 1
      i32.const 12
      i32.shr_u
      i32.const 63
      i32.and
      i32.const 128
      i32.or
      i32.store8 offset=13
      local.get 2
      local.get 1
      i32.const 18
      i32.shr_u
      i32.const 7
      i32.and
      i32.const 240
      i32.or
      i32.store8 offset=12
      i32.const 4
      local.set 1
    end
    local.get 0
    local.get 2
    i32.const 12
    i32.add
    local.get 1
    call 49
    local.set 1
    local.get 2
    i32.const 16
    i32.add
    global.set 0
    local.get 1)
  (func (;51;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    local.get 0
    i32.store offset=4
    local.get 2
    i32.const 8
    i32.add
    i32.const 16
    i32.add
    local.get 1
    i32.const 16
    i32.add
    i64.load align=4
    i64.store
    local.get 2
    i32.const 8
    i32.add
    i32.const 8
    i32.add
    local.get 1
    i32.const 8
    i32.add
    i64.load align=4
    i64.store
    local.get 2
    local.get 1
    i64.load align=4
    i64.store offset=8
    local.get 2
    i32.const 4
    i32.add
    i32.const 1049960
    local.get 2
    i32.const 8
    i32.add
    call 45
    local.set 1
    local.get 2
    i32.const 32
    i32.add
    global.set 0
    local.get 1)
  (func (;52;) (type 1) (param i32 i32 i32) (result i32)
    local.get 0
    i32.load
    local.get 1
    local.get 2
    call 49)
  (func (;53;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 2
    global.set 0
    local.get 0
    i32.load
    local.set 0
    local.get 2
    i32.const 0
    i32.store offset=12
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.const 128
            i32.lt_u
            br_if 0 (;@4;)
            local.get 1
            i32.const 2048
            i32.lt_u
            br_if 1 (;@3;)
            local.get 1
            i32.const 65536
            i32.ge_u
            br_if 2 (;@2;)
            local.get 2
            local.get 1
            i32.const 63
            i32.and
            i32.const 128
            i32.or
            i32.store8 offset=14
            local.get 2
            local.get 1
            i32.const 12
            i32.shr_u
            i32.const 224
            i32.or
            i32.store8 offset=12
            local.get 2
            local.get 1
            i32.const 6
            i32.shr_u
            i32.const 63
            i32.and
            i32.const 128
            i32.or
            i32.store8 offset=13
            i32.const 3
            local.set 1
            br 3 (;@1;)
          end
          local.get 2
          local.get 1
          i32.store8 offset=12
          i32.const 1
          local.set 1
          br 2 (;@1;)
        end
        local.get 2
        local.get 1
        i32.const 63
        i32.and
        i32.const 128
        i32.or
        i32.store8 offset=13
        local.get 2
        local.get 1
        i32.const 6
        i32.shr_u
        i32.const 192
        i32.or
        i32.store8 offset=12
        i32.const 2
        local.set 1
        br 1 (;@1;)
      end
      local.get 2
      local.get 1
      i32.const 63
      i32.and
      i32.const 128
      i32.or
      i32.store8 offset=15
      local.get 2
      local.get 1
      i32.const 6
      i32.shr_u
      i32.const 63
      i32.and
      i32.const 128
      i32.or
      i32.store8 offset=14
      local.get 2
      local.get 1
      i32.const 12
      i32.shr_u
      i32.const 63
      i32.and
      i32.const 128
      i32.or
      i32.store8 offset=13
      local.get 2
      local.get 1
      i32.const 18
      i32.shr_u
      i32.const 7
      i32.and
      i32.const 240
      i32.or
      i32.store8 offset=12
      i32.const 4
      local.set 1
    end
    local.get 0
    local.get 2
    i32.const 12
    i32.add
    local.get 1
    call 49
    local.set 1
    local.get 2
    i32.const 16
    i32.add
    global.set 0
    local.get 1)
  (func (;54;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 2
    global.set 0
    local.get 0
    i32.load
    local.set 0
    local.get 2
    i32.const 8
    i32.add
    i32.const 16
    i32.add
    local.get 1
    i32.const 16
    i32.add
    i64.load align=4
    i64.store
    local.get 2
    i32.const 8
    i32.add
    i32.const 8
    i32.add
    local.get 1
    i32.const 8
    i32.add
    i64.load align=4
    i64.store
    local.get 2
    local.get 1
    i64.load align=4
    i64.store offset=8
    local.get 2
    local.get 0
    i32.store offset=4
    local.get 2
    i32.const 4
    i32.add
    i32.const 1049960
    local.get 2
    i32.const 8
    i32.add
    call 45
    local.set 1
    local.get 2
    i32.const 32
    i32.add
    global.set 0
    local.get 1)
  (func (;55;) (type 1) (param i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i64)
    global.get 0
    i32.const 64
    i32.sub
    local.tee 3
    global.set 0
    block  ;; label = @1
      block  ;; label = @2
        local.get 0
        i32.load8_u offset=8
        i32.eqz
        br_if 0 (;@2;)
        local.get 0
        i32.load
        local.set 4
        i32.const 1
        local.set 5
        br 1 (;@1;)
      end
      local.get 0
      i32.load
      local.set 4
      block  ;; label = @2
        local.get 0
        i32.const 4
        i32.add
        i32.load
        local.tee 6
        i32.load offset=28
        local.tee 7
        i32.const 4
        i32.and
        br_if 0 (;@2;)
        i32.const 1
        local.set 5
        local.get 6
        i32.load offset=20
        i32.const 1049700
        i32.const 1049704
        local.get 4
        select
        i32.const 2
        i32.const 1
        local.get 4
        select
        local.get 6
        i32.const 24
        i32.add
        i32.load
        i32.load offset=12
        call_indirect (type 1)
        br_if 1 (;@1;)
        local.get 1
        local.get 6
        local.get 2
        i32.load offset=12
        call_indirect (type 2)
        local.set 5
        br 1 (;@1;)
      end
      block  ;; label = @2
        local.get 4
        br_if 0 (;@2;)
        block  ;; label = @3
          local.get 6
          i32.load offset=20
          i32.const 1049705
          i32.const 2
          local.get 6
          i32.const 24
          i32.add
          i32.load
          i32.load offset=12
          call_indirect (type 1)
          i32.eqz
          br_if 0 (;@3;)
          i32.const 1
          local.set 5
          i32.const 0
          local.set 4
          br 2 (;@1;)
        end
        local.get 6
        i32.load offset=28
        local.set 7
      end
      i32.const 1
      local.set 5
      local.get 3
      i32.const 1
      i32.store8 offset=23
      local.get 3
      i32.const 48
      i32.add
      i32.const 1049672
      i32.store
      local.get 3
      local.get 6
      i64.load offset=20 align=4
      i64.store offset=8
      local.get 3
      local.get 3
      i32.const 23
      i32.add
      i32.store offset=16
      local.get 3
      local.get 6
      i64.load offset=8 align=4
      i64.store offset=32
      local.get 6
      i64.load align=4
      local.set 8
      local.get 3
      local.get 7
      i32.store offset=52
      local.get 3
      local.get 6
      i32.load offset=16
      i32.store offset=40
      local.get 3
      local.get 6
      i32.load8_u offset=32
      i32.store8 offset=56
      local.get 3
      local.get 8
      i64.store offset=24
      local.get 3
      local.get 3
      i32.const 8
      i32.add
      i32.store offset=44
      local.get 1
      local.get 3
      i32.const 24
      i32.add
      local.get 2
      i32.load offset=12
      call_indirect (type 2)
      br_if 0 (;@1;)
      local.get 3
      i32.load offset=44
      i32.const 1049702
      i32.const 2
      local.get 3
      i32.load offset=48
      i32.load offset=12
      call_indirect (type 1)
      local.set 5
    end
    local.get 0
    local.get 5
    i32.store8 offset=8
    local.get 0
    local.get 4
    i32.const 1
    i32.add
    i32.store
    local.get 3
    i32.const 64
    i32.add
    global.set 0
    local.get 0)
  (func (;56;) (type 8) (param i32 i32 i32 i32 i32)
    local.get 0
    local.get 1
    local.get 2
    local.get 3
    local.get 4
    call 57
    unreachable)
  (func (;57;) (type 8) (param i32 i32 i32 i32 i32)
    (local i32 i32 i32 i32 i32)
    global.get 0
    i32.const 112
    i32.sub
    local.tee 5
    global.set 0
    local.get 5
    local.get 3
    i32.store offset=12
    local.get 5
    local.get 2
    i32.store offset=8
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 1
          i32.const 257
          i32.lt_u
          br_if 0 (;@3;)
          i32.const 256
          local.set 6
          block  ;; label = @4
            local.get 0
            i32.load8_s offset=256
            i32.const -65
            i32.gt_s
            br_if 0 (;@4;)
            i32.const 255
            local.set 6
            local.get 0
            i32.load8_s offset=255
            i32.const -65
            i32.gt_s
            br_if 0 (;@4;)
            i32.const 254
            local.set 6
            local.get 0
            i32.load8_s offset=254
            i32.const -65
            i32.gt_s
            br_if 0 (;@4;)
            i32.const 253
            local.set 6
            local.get 0
            i32.load8_s offset=253
            i32.const -65
            i32.le_s
            br_if 2 (;@2;)
          end
          local.get 5
          local.get 6
          i32.store offset=20
          local.get 5
          local.get 0
          i32.store offset=16
          i32.const 5
          local.set 6
          i32.const 1050172
          local.set 7
          br 2 (;@1;)
        end
        local.get 5
        local.get 1
        i32.store offset=20
        local.get 5
        local.get 0
        i32.store offset=16
        i32.const 0
        local.set 6
        i32.const 1053004
        local.set 7
        br 1 (;@1;)
      end
      local.get 0
      local.get 1
      i32.const 0
      i32.const 253
      local.get 4
      call 56
      unreachable
    end
    local.get 5
    local.get 6
    i32.store offset=28
    local.get 5
    local.get 7
    i32.store offset=24
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 2
            local.get 1
            i32.gt_u
            local.tee 6
            br_if 0 (;@4;)
            local.get 3
            local.get 1
            i32.gt_u
            br_if 0 (;@4;)
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 2
                    local.get 3
                    i32.gt_u
                    br_if 0 (;@8;)
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 2
                        i32.eqz
                        br_if 0 (;@10;)
                        block  ;; label = @11
                          local.get 2
                          local.get 1
                          i32.lt_u
                          br_if 0 (;@11;)
                          local.get 2
                          local.get 1
                          i32.eq
                          br_if 1 (;@10;)
                          br 2 (;@9;)
                        end
                        local.get 0
                        local.get 2
                        i32.add
                        i32.load8_s
                        i32.const -64
                        i32.lt_s
                        br_if 1 (;@9;)
                      end
                      local.get 3
                      local.set 2
                    end
                    local.get 5
                    local.get 2
                    i32.store offset=32
                    local.get 1
                    local.set 3
                    block  ;; label = @9
                      local.get 2
                      local.get 1
                      i32.ge_u
                      br_if 0 (;@9;)
                      local.get 2
                      i32.const 1
                      i32.add
                      local.tee 6
                      i32.const 0
                      local.get 2
                      i32.const -3
                      i32.add
                      local.tee 3
                      local.get 3
                      local.get 2
                      i32.gt_u
                      select
                      local.tee 3
                      i32.lt_u
                      br_if 6 (;@3;)
                      block  ;; label = @10
                        local.get 3
                        local.get 6
                        i32.eq
                        br_if 0 (;@10;)
                        local.get 0
                        local.get 6
                        i32.add
                        local.get 0
                        local.get 3
                        i32.add
                        local.tee 8
                        i32.sub
                        local.set 6
                        block  ;; label = @11
                          local.get 0
                          local.get 2
                          i32.add
                          local.tee 9
                          i32.load8_s
                          i32.const -65
                          i32.le_s
                          br_if 0 (;@11;)
                          local.get 6
                          i32.const -1
                          i32.add
                          local.set 7
                          br 1 (;@10;)
                        end
                        local.get 3
                        local.get 2
                        i32.eq
                        br_if 0 (;@10;)
                        block  ;; label = @11
                          local.get 9
                          i32.const -1
                          i32.add
                          local.tee 2
                          i32.load8_s
                          i32.const -65
                          i32.le_s
                          br_if 0 (;@11;)
                          local.get 6
                          i32.const -2
                          i32.add
                          local.set 7
                          br 1 (;@10;)
                        end
                        local.get 8
                        local.get 2
                        i32.eq
                        br_if 0 (;@10;)
                        block  ;; label = @11
                          local.get 2
                          i32.const -1
                          i32.add
                          local.tee 2
                          i32.load8_s
                          i32.const -65
                          i32.le_s
                          br_if 0 (;@11;)
                          local.get 6
                          i32.const -3
                          i32.add
                          local.set 7
                          br 1 (;@10;)
                        end
                        local.get 8
                        local.get 2
                        i32.eq
                        br_if 0 (;@10;)
                        block  ;; label = @11
                          local.get 2
                          i32.const -1
                          i32.add
                          local.tee 2
                          i32.load8_s
                          i32.const -65
                          i32.le_s
                          br_if 0 (;@11;)
                          local.get 6
                          i32.const -4
                          i32.add
                          local.set 7
                          br 1 (;@10;)
                        end
                        local.get 8
                        local.get 2
                        i32.eq
                        br_if 0 (;@10;)
                        local.get 6
                        i32.const -5
                        i32.add
                        local.set 7
                      end
                      local.get 7
                      local.get 3
                      i32.add
                      local.set 3
                    end
                    block  ;; label = @9
                      local.get 3
                      i32.eqz
                      br_if 0 (;@9;)
                      block  ;; label = @10
                        block  ;; label = @11
                          local.get 3
                          local.get 1
                          i32.lt_u
                          br_if 0 (;@11;)
                          local.get 1
                          local.get 3
                          i32.eq
                          br_if 1 (;@10;)
                          br 10 (;@1;)
                        end
                        local.get 0
                        local.get 3
                        i32.add
                        i32.load8_s
                        i32.const -65
                        i32.le_s
                        br_if 9 (;@1;)
                      end
                      local.get 1
                      local.get 3
                      i32.sub
                      local.set 1
                    end
                    local.get 1
                    i32.eqz
                    br_if 6 (;@2;)
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 0
                        local.get 3
                        i32.add
                        local.tee 1
                        i32.load8_s
                        local.tee 2
                        i32.const -1
                        i32.gt_s
                        br_if 0 (;@10;)
                        local.get 1
                        i32.load8_u offset=1
                        i32.const 63
                        i32.and
                        local.set 0
                        local.get 2
                        i32.const 31
                        i32.and
                        local.set 6
                        local.get 2
                        i32.const -33
                        i32.gt_u
                        br_if 1 (;@9;)
                        local.get 6
                        i32.const 6
                        i32.shl
                        local.get 0
                        i32.or
                        local.set 1
                        br 4 (;@6;)
                      end
                      local.get 5
                      local.get 2
                      i32.const 255
                      i32.and
                      i32.store offset=36
                      i32.const 1
                      local.set 2
                      br 4 (;@5;)
                    end
                    local.get 0
                    i32.const 6
                    i32.shl
                    local.get 1
                    i32.load8_u offset=2
                    i32.const 63
                    i32.and
                    i32.or
                    local.set 0
                    local.get 2
                    i32.const -16
                    i32.ge_u
                    br_if 1 (;@7;)
                    local.get 0
                    local.get 6
                    i32.const 12
                    i32.shl
                    i32.or
                    local.set 1
                    br 2 (;@6;)
                  end
                  local.get 5
                  i32.const 100
                  i32.add
                  i32.const 2
                  i32.store
                  local.get 5
                  i32.const 92
                  i32.add
                  i32.const 2
                  i32.store
                  local.get 5
                  i32.const 72
                  i32.add
                  i32.const 12
                  i32.add
                  i32.const 4
                  i32.store
                  local.get 5
                  i32.const 48
                  i32.add
                  i32.const 12
                  i32.add
                  i64.const 4
                  i64.store align=4
                  local.get 5
                  i32.const 4
                  i32.store offset=52
                  local.get 5
                  i32.const 1050316
                  i32.store offset=48
                  local.get 5
                  i32.const 4
                  i32.store offset=76
                  local.get 5
                  local.get 5
                  i32.const 72
                  i32.add
                  i32.store offset=56
                  local.get 5
                  local.get 5
                  i32.const 24
                  i32.add
                  i32.store offset=96
                  local.get 5
                  local.get 5
                  i32.const 16
                  i32.add
                  i32.store offset=88
                  local.get 5
                  local.get 5
                  i32.const 12
                  i32.add
                  i32.store offset=80
                  local.get 5
                  local.get 5
                  i32.const 8
                  i32.add
                  i32.store offset=72
                  local.get 5
                  i32.const 48
                  i32.add
                  local.get 4
                  call 24
                  unreachable
                end
                local.get 0
                i32.const 6
                i32.shl
                local.get 1
                i32.load8_u offset=3
                i32.const 63
                i32.and
                i32.or
                local.get 6
                i32.const 18
                i32.shl
                i32.const 1835008
                i32.and
                i32.or
                local.tee 1
                i32.const 1114112
                i32.eq
                br_if 4 (;@2;)
              end
              local.get 5
              local.get 1
              i32.store offset=36
              i32.const 1
              local.set 2
              local.get 1
              i32.const 128
              i32.lt_u
              br_if 0 (;@5;)
              i32.const 2
              local.set 2
              local.get 1
              i32.const 2048
              i32.lt_u
              br_if 0 (;@5;)
              i32.const 3
              i32.const 4
              local.get 1
              i32.const 65536
              i32.lt_u
              select
              local.set 2
            end
            local.get 5
            local.get 3
            i32.store offset=40
            local.get 5
            local.get 2
            local.get 3
            i32.add
            i32.store offset=44
            local.get 5
            i32.const 48
            i32.add
            i32.const 12
            i32.add
            i64.const 5
            i64.store align=4
            local.get 5
            i32.const 108
            i32.add
            i32.const 2
            i32.store
            local.get 5
            i32.const 100
            i32.add
            i32.const 2
            i32.store
            local.get 5
            i32.const 92
            i32.add
            i32.const 6
            i32.store
            local.get 5
            i32.const 72
            i32.add
            i32.const 12
            i32.add
            i32.const 7
            i32.store
            local.get 5
            i32.const 5
            i32.store offset=52
            local.get 5
            i32.const 1050240
            i32.store offset=48
            local.get 5
            i32.const 4
            i32.store offset=76
            local.get 5
            local.get 5
            i32.const 72
            i32.add
            i32.store offset=56
            local.get 5
            local.get 5
            i32.const 24
            i32.add
            i32.store offset=104
            local.get 5
            local.get 5
            i32.const 16
            i32.add
            i32.store offset=96
            local.get 5
            local.get 5
            i32.const 40
            i32.add
            i32.store offset=88
            local.get 5
            local.get 5
            i32.const 36
            i32.add
            i32.store offset=80
            local.get 5
            local.get 5
            i32.const 32
            i32.add
            i32.store offset=72
            local.get 5
            i32.const 48
            i32.add
            local.get 4
            call 24
            unreachable
          end
          local.get 5
          local.get 2
          local.get 3
          local.get 6
          select
          i32.store offset=40
          local.get 5
          i32.const 48
          i32.add
          i32.const 12
          i32.add
          i64.const 3
          i64.store align=4
          local.get 5
          i32.const 92
          i32.add
          i32.const 2
          i32.store
          local.get 5
          i32.const 72
          i32.add
          i32.const 12
          i32.add
          i32.const 2
          i32.store
          local.get 5
          i32.const 3
          i32.store offset=52
          local.get 5
          i32.const 1050372
          i32.store offset=48
          local.get 5
          i32.const 4
          i32.store offset=76
          local.get 5
          local.get 5
          i32.const 72
          i32.add
          i32.store offset=56
          local.get 5
          local.get 5
          i32.const 24
          i32.add
          i32.store offset=88
          local.get 5
          local.get 5
          i32.const 16
          i32.add
          i32.store offset=80
          local.get 5
          local.get 5
          i32.const 40
          i32.add
          i32.store offset=72
          local.get 5
          i32.const 48
          i32.add
          local.get 4
          call 24
          unreachable
        end
        local.get 3
        local.get 6
        i32.const 1050424
        call 47
        unreachable
      end
      i32.const 1052959
      i32.const 43
      local.get 4
      call 22
      unreachable
    end
    local.get 0
    local.get 1
    local.get 3
    local.get 1
    local.get 4
    call 56
    unreachable)
  (func (;58;) (type 10) (param i32 i32 i32)
    (local i32 i32 i32 i32 i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 3
    global.set 0
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 1
                        br_table 5 (;@5;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 1 (;@9;) 3 (;@7;) 8 (;@2;) 8 (;@2;) 2 (;@8;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 6 (;@4;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 8 (;@2;) 7 (;@3;) 0 (;@10;)
                      end
                      local.get 1
                      i32.const 92
                      i32.eq
                      br_if 3 (;@6;)
                      br 7 (;@2;)
                    end
                    local.get 0
                    i32.const 512
                    i32.store16 offset=10
                    local.get 0
                    i64.const 0
                    i64.store offset=2 align=2
                    local.get 0
                    i32.const 29788
                    i32.store16
                    br 7 (;@1;)
                  end
                  local.get 0
                  i32.const 512
                  i32.store16 offset=10
                  local.get 0
                  i64.const 0
                  i64.store offset=2 align=2
                  local.get 0
                  i32.const 29276
                  i32.store16
                  br 6 (;@1;)
                end
                local.get 0
                i32.const 512
                i32.store16 offset=10
                local.get 0
                i64.const 0
                i64.store offset=2 align=2
                local.get 0
                i32.const 28252
                i32.store16
                br 5 (;@1;)
              end
              local.get 0
              i32.const 512
              i32.store16 offset=10
              local.get 0
              i64.const 0
              i64.store offset=2 align=2
              local.get 0
              i32.const 23644
              i32.store16
              br 4 (;@1;)
            end
            local.get 0
            i32.const 512
            i32.store16 offset=10
            local.get 0
            i64.const 0
            i64.store offset=2 align=2
            local.get 0
            i32.const 12380
            i32.store16
            br 3 (;@1;)
          end
          local.get 2
          i32.const 65536
          i32.and
          i32.eqz
          br_if 1 (;@2;)
          local.get 0
          i32.const 512
          i32.store16 offset=10
          local.get 0
          i64.const 0
          i64.store offset=2 align=2
          local.get 0
          i32.const 8796
          i32.store16
          br 2 (;@1;)
        end
        local.get 2
        i32.const 256
        i32.and
        i32.eqz
        br_if 0 (;@2;)
        local.get 0
        i32.const 512
        i32.store16 offset=10
        local.get 0
        i64.const 0
        i64.store offset=2 align=2
        local.get 0
        i32.const 10076
        i32.store16
        br 1 (;@1;)
      end
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    block  ;; label = @9
                      local.get 2
                      i32.const 1
                      i32.and
                      i32.eqz
                      br_if 0 (;@9;)
                      local.get 1
                      i32.const 11
                      i32.shl
                      local.set 4
                      i32.const 0
                      local.set 2
                      i32.const 33
                      local.set 5
                      i32.const 33
                      local.set 6
                      block  ;; label = @10
                        block  ;; label = @11
                          loop  ;; label = @12
                            block  ;; label = @13
                              block  ;; label = @14
                                i32.const -1
                                local.get 5
                                i32.const 1
                                i32.shr_u
                                local.get 2
                                i32.add
                                local.tee 5
                                i32.const 2
                                i32.shl
                                i32.const 1052100
                                i32.add
                                i32.load
                                i32.const 11
                                i32.shl
                                local.tee 7
                                local.get 4
                                i32.ne
                                local.get 7
                                local.get 4
                                i32.lt_u
                                select
                                local.tee 7
                                i32.const 1
                                i32.ne
                                br_if 0 (;@14;)
                                local.get 5
                                local.set 6
                                br 1 (;@13;)
                              end
                              local.get 7
                              i32.const 255
                              i32.and
                              i32.const 255
                              i32.ne
                              br_if 2 (;@11;)
                              local.get 5
                              i32.const 1
                              i32.add
                              local.set 2
                            end
                            local.get 6
                            local.get 2
                            i32.sub
                            local.set 5
                            local.get 6
                            local.get 2
                            i32.gt_u
                            br_if 0 (;@12;)
                            br 2 (;@10;)
                          end
                        end
                        local.get 5
                        i32.const 1
                        i32.add
                        local.set 2
                      end
                      block  ;; label = @10
                        block  ;; label = @11
                          block  ;; label = @12
                            block  ;; label = @13
                              block  ;; label = @14
                                local.get 2
                                i32.const 32
                                i32.gt_u
                                br_if 0 (;@14;)
                                local.get 2
                                i32.const 2
                                i32.shl
                                local.tee 4
                                i32.const 1052100
                                i32.add
                                i32.load
                                i32.const 21
                                i32.shr_u
                                local.set 6
                                local.get 2
                                i32.const 32
                                i32.ne
                                br_if 1 (;@13;)
                                i32.const 727
                                local.set 7
                                i32.const 31
                                local.set 2
                                br 2 (;@12;)
                              end
                              i32.const 33
                              i32.const 33
                              i32.const 1051956
                              call 34
                              unreachable
                            end
                            local.get 4
                            i32.const 1052104
                            i32.add
                            i32.load
                            i32.const 21
                            i32.shr_u
                            local.set 7
                            local.get 2
                            i32.eqz
                            br_if 1 (;@11;)
                            local.get 2
                            i32.const -1
                            i32.add
                            local.set 2
                          end
                          local.get 2
                          i32.const 2
                          i32.shl
                          i32.const 1052100
                          i32.add
                          i32.load
                          i32.const 2097151
                          i32.and
                          local.set 2
                          br 1 (;@10;)
                        end
                        i32.const 0
                        local.set 2
                      end
                      block  ;; label = @10
                        local.get 7
                        local.get 6
                        i32.const -1
                        i32.xor
                        i32.add
                        i32.eqz
                        br_if 0 (;@10;)
                        local.get 1
                        local.get 2
                        i32.sub
                        local.set 5
                        local.get 6
                        i32.const 727
                        local.get 6
                        i32.const 727
                        i32.gt_u
                        select
                        local.set 4
                        local.get 7
                        i32.const -1
                        i32.add
                        local.set 7
                        i32.const 0
                        local.set 2
                        loop  ;; label = @11
                          local.get 4
                          local.get 6
                          i32.eq
                          br_if 7 (;@4;)
                          local.get 2
                          local.get 6
                          i32.const 1052232
                          i32.add
                          i32.load8_u
                          i32.add
                          local.tee 2
                          local.get 5
                          i32.gt_u
                          br_if 1 (;@10;)
                          local.get 7
                          local.get 6
                          i32.const 1
                          i32.add
                          local.tee 6
                          i32.ne
                          br_if 0 (;@11;)
                        end
                        local.get 7
                        local.set 6
                      end
                      local.get 6
                      i32.const 1
                      i32.and
                      br_if 1 (;@8;)
                    end
                    local.get 1
                    i32.const 32
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const 127
                    i32.lt_u
                    br_if 3 (;@5;)
                    local.get 1
                    i32.const 65536
                    i32.lt_u
                    br_if 2 (;@6;)
                    local.get 1
                    i32.const 131072
                    i32.lt_u
                    br_if 1 (;@7;)
                    local.get 1
                    i32.const -918000
                    i32.add
                    i32.const 196112
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -205744
                    i32.add
                    i32.const 712016
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -201547
                    i32.add
                    i32.const 5
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -195102
                    i32.add
                    i32.const 1506
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -191457
                    i32.add
                    i32.const 3103
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -183970
                    i32.add
                    i32.const 14
                    i32.lt_u
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -2
                    i32.and
                    i32.const 178206
                    i32.eq
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -32
                    i32.and
                    i32.const 173792
                    i32.eq
                    br_if 6 (;@2;)
                    local.get 1
                    i32.const -177978
                    i32.add
                    i32.const 6
                    i32.lt_u
                    br_if 6 (;@2;)
                    br 3 (;@5;)
                  end
                  local.get 3
                  i32.const 6
                  i32.add
                  i32.const 2
                  i32.add
                  i32.const 0
                  i32.store8
                  local.get 3
                  i32.const 0
                  i32.store16 offset=6
                  local.get 3
                  i32.const 125
                  i32.store8 offset=15
                  local.get 3
                  local.get 1
                  i32.const 15
                  i32.and
                  i32.const 1051988
                  i32.add
                  i32.load8_u
                  i32.store8 offset=14
                  local.get 3
                  local.get 1
                  i32.const 4
                  i32.shr_u
                  i32.const 15
                  i32.and
                  i32.const 1051988
                  i32.add
                  i32.load8_u
                  i32.store8 offset=13
                  local.get 3
                  local.get 1
                  i32.const 8
                  i32.shr_u
                  i32.const 15
                  i32.and
                  i32.const 1051988
                  i32.add
                  i32.load8_u
                  i32.store8 offset=12
                  local.get 3
                  local.get 1
                  i32.const 12
                  i32.shr_u
                  i32.const 15
                  i32.and
                  i32.const 1051988
                  i32.add
                  i32.load8_u
                  i32.store8 offset=11
                  local.get 3
                  local.get 1
                  i32.const 16
                  i32.shr_u
                  i32.const 15
                  i32.and
                  i32.const 1051988
                  i32.add
                  i32.load8_u
                  i32.store8 offset=10
                  local.get 3
                  local.get 1
                  i32.const 20
                  i32.shr_u
                  i32.const 15
                  i32.and
                  i32.const 1051988
                  i32.add
                  i32.load8_u
                  i32.store8 offset=9
                  local.get 1
                  i32.const 1
                  i32.or
                  i32.clz
                  i32.const 2
                  i32.shr_u
                  i32.const -2
                  i32.add
                  local.tee 2
                  i32.const 11
                  i32.ge_u
                  br_if 4 (;@3;)
                  local.get 3
                  i32.const 6
                  i32.add
                  local.get 2
                  i32.add
                  local.tee 6
                  i32.const 0
                  i32.load16_u offset=1052048 align=1
                  i32.store16 align=1
                  local.get 6
                  i32.const 2
                  i32.add
                  i32.const 0
                  i32.load8_u offset=1052050
                  i32.store8
                  local.get 0
                  local.get 3
                  i64.load offset=6 align=2
                  i64.store align=1
                  local.get 0
                  i32.const 8
                  i32.add
                  local.get 3
                  i32.const 6
                  i32.add
                  i32.const 8
                  i32.add
                  i32.load16_u
                  i32.store16 align=1
                  local.get 0
                  i32.const 10
                  i32.store8 offset=11
                  local.get 0
                  local.get 2
                  i32.store8 offset=10
                  br 6 (;@1;)
                end
                local.get 1
                i32.const 1050512
                i32.const 44
                i32.const 1050600
                i32.const 196
                i32.const 1050796
                i32.const 450
                call 59
                br_if 1 (;@5;)
                br 4 (;@2;)
              end
              local.get 1
              i32.const 1051246
              i32.const 40
              i32.const 1051326
              i32.const 287
              i32.const 1051613
              i32.const 303
              call 59
              i32.eqz
              br_if 3 (;@2;)
            end
            local.get 0
            local.get 1
            i32.store offset=4
            local.get 0
            i32.const 128
            i32.store8
            br 3 (;@1;)
          end
          local.get 4
          i32.const 727
          i32.const 1051972
          call 34
          unreachable
        end
        local.get 2
        i32.const 10
        i32.const 1052032
        call 39
        unreachable
      end
      local.get 3
      i32.const 6
      i32.add
      i32.const 2
      i32.add
      i32.const 0
      i32.store8
      local.get 3
      i32.const 0
      i32.store16 offset=6
      local.get 3
      i32.const 125
      i32.store8 offset=15
      local.get 3
      local.get 1
      i32.const 15
      i32.and
      i32.const 1051988
      i32.add
      i32.load8_u
      i32.store8 offset=14
      local.get 3
      local.get 1
      i32.const 4
      i32.shr_u
      i32.const 15
      i32.and
      i32.const 1051988
      i32.add
      i32.load8_u
      i32.store8 offset=13
      local.get 3
      local.get 1
      i32.const 8
      i32.shr_u
      i32.const 15
      i32.and
      i32.const 1051988
      i32.add
      i32.load8_u
      i32.store8 offset=12
      local.get 3
      local.get 1
      i32.const 12
      i32.shr_u
      i32.const 15
      i32.and
      i32.const 1051988
      i32.add
      i32.load8_u
      i32.store8 offset=11
      local.get 3
      local.get 1
      i32.const 16
      i32.shr_u
      i32.const 15
      i32.and
      i32.const 1051988
      i32.add
      i32.load8_u
      i32.store8 offset=10
      local.get 3
      local.get 1
      i32.const 20
      i32.shr_u
      i32.const 15
      i32.and
      i32.const 1051988
      i32.add
      i32.load8_u
      i32.store8 offset=9
      block  ;; label = @2
        local.get 1
        i32.const 1
        i32.or
        i32.clz
        i32.const 2
        i32.shr_u
        i32.const -2
        i32.add
        local.tee 2
        i32.const 11
        i32.ge_u
        br_if 0 (;@2;)
        local.get 3
        i32.const 6
        i32.add
        local.get 2
        i32.add
        local.tee 6
        i32.const 0
        i32.load16_u offset=1052048 align=1
        i32.store16 align=1
        local.get 6
        i32.const 2
        i32.add
        i32.const 0
        i32.load8_u offset=1052050
        i32.store8
        local.get 0
        local.get 3
        i64.load offset=6 align=2
        i64.store align=1
        local.get 0
        i32.const 8
        i32.add
        local.get 3
        i32.const 6
        i32.add
        i32.const 8
        i32.add
        i32.load16_u
        i32.store16 align=1
        local.get 0
        i32.const 10
        i32.store8 offset=11
        local.get 0
        local.get 2
        i32.store8 offset=10
        br 1 (;@1;)
      end
      local.get 2
      i32.const 10
      i32.const 1052032
      call 39
      unreachable
    end
    local.get 3
    i32.const 16
    i32.add
    global.set 0)
  (func (;59;) (type 20) (param i32 i32 i32 i32 i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32)
    local.get 1
    local.get 2
    i32.const 1
    i32.shl
    i32.add
    local.set 7
    local.get 0
    i32.const 65280
    i32.and
    i32.const 8
    i32.shr_u
    local.set 8
    i32.const 0
    local.set 9
    local.get 0
    i32.const 255
    i32.and
    local.set 10
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            loop  ;; label = @5
              local.get 1
              i32.const 2
              i32.add
              local.set 11
              local.get 9
              local.get 1
              i32.load8_u offset=1
              local.tee 2
              i32.add
              local.set 12
              block  ;; label = @6
                local.get 1
                i32.load8_u
                local.tee 1
                local.get 8
                i32.eq
                br_if 0 (;@6;)
                local.get 1
                local.get 8
                i32.gt_u
                br_if 4 (;@2;)
                local.get 12
                local.set 9
                local.get 11
                local.set 1
                local.get 11
                local.get 7
                i32.ne
                br_if 1 (;@5;)
                br 4 (;@2;)
              end
              local.get 12
              local.get 9
              i32.lt_u
              br_if 1 (;@4;)
              local.get 12
              local.get 4
              i32.gt_u
              br_if 2 (;@3;)
              local.get 3
              local.get 9
              i32.add
              local.set 1
              loop  ;; label = @6
                block  ;; label = @7
                  local.get 2
                  br_if 0 (;@7;)
                  local.get 12
                  local.set 9
                  local.get 11
                  local.set 1
                  local.get 11
                  local.get 7
                  i32.ne
                  br_if 2 (;@5;)
                  br 5 (;@2;)
                end
                local.get 2
                i32.const -1
                i32.add
                local.set 2
                local.get 1
                i32.load8_u
                local.set 9
                local.get 1
                i32.const 1
                i32.add
                local.set 1
                local.get 9
                local.get 10
                i32.ne
                br_if 0 (;@6;)
              end
            end
            i32.const 0
            local.set 2
            br 3 (;@1;)
          end
          local.get 9
          local.get 12
          i32.const 1050496
          call 47
          unreachable
        end
        local.get 12
        local.get 4
        call 40
        unreachable
      end
      local.get 0
      i32.const 65535
      i32.and
      local.set 9
      local.get 5
      local.get 6
      i32.add
      local.set 12
      i32.const 1
      local.set 2
      loop  ;; label = @2
        local.get 5
        i32.const 1
        i32.add
        local.set 10
        block  ;; label = @3
          block  ;; label = @4
            local.get 5
            i32.load8_u
            local.tee 1
            i32.extend8_s
            local.tee 11
            i32.const 0
            i32.lt_s
            br_if 0 (;@4;)
            local.get 10
            local.set 5
            br 1 (;@3;)
          end
          block  ;; label = @4
            local.get 10
            local.get 12
            i32.eq
            br_if 0 (;@4;)
            local.get 11
            i32.const 127
            i32.and
            i32.const 8
            i32.shl
            local.get 5
            i32.load8_u offset=1
            i32.or
            local.set 1
            local.get 5
            i32.const 2
            i32.add
            local.set 5
            br 1 (;@3;)
          end
          i32.const 1052959
          i32.const 43
          i32.const 1050480
          call 22
          unreachable
        end
        local.get 9
        local.get 1
        i32.sub
        local.tee 9
        i32.const 0
        i32.lt_s
        br_if 1 (;@1;)
        local.get 2
        i32.const 1
        i32.xor
        local.set 2
        local.get 5
        local.get 12
        i32.ne
        br_if 0 (;@2;)
      end
    end
    local.get 2
    i32.const 1
    i32.and)
  (func (;60;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 2
    global.set 0
    i32.const 1
    local.set 3
    block  ;; label = @1
      local.get 0
      i32.load
      local.get 1
      call 48
      br_if 0 (;@1;)
      local.get 1
      i32.const 24
      i32.add
      i32.load
      local.set 4
      local.get 1
      i32.load offset=20
      local.set 5
      local.get 2
      i64.const 0
      i64.store offset=20 align=4
      local.get 2
      i32.const 1053004
      i32.store offset=16
      i32.const 1
      local.set 3
      local.get 2
      i32.const 1
      i32.store offset=12
      local.get 2
      i32.const 1049428
      i32.store offset=8
      local.get 5
      local.get 4
      local.get 2
      i32.const 8
      i32.add
      call 45
      br_if 0 (;@1;)
      local.get 0
      i32.load offset=4
      local.get 1
      call 48
      local.set 3
    end
    local.get 2
    i32.const 32
    i32.add
    global.set 0
    local.get 3)
  (func (;61;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 2
    global.set 0
    i32.const 1
    local.set 3
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.load offset=20
        local.tee 4
        i32.const 39
        local.get 1
        i32.const 24
        i32.add
        i32.load
        i32.load offset=16
        local.tee 5
        call_indirect (type 2)
        br_if 0 (;@2;)
        local.get 2
        local.get 0
        i32.load
        i32.const 257
        call 58
        block  ;; label = @3
          block  ;; label = @4
            local.get 2
            i32.load8_u
            i32.const 128
            i32.ne
            br_if 0 (;@4;)
            local.get 2
            i32.const 8
            i32.add
            local.set 6
            i32.const 128
            local.set 7
            loop  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  local.get 7
                  i32.const 255
                  i32.and
                  i32.const 128
                  i32.eq
                  br_if 0 (;@7;)
                  local.get 2
                  i32.load8_u offset=10
                  local.tee 0
                  local.get 2
                  i32.load8_u offset=11
                  i32.ge_u
                  br_if 4 (;@3;)
                  local.get 2
                  local.get 0
                  i32.const 1
                  i32.add
                  i32.store8 offset=10
                  local.get 0
                  i32.const 10
                  i32.ge_u
                  br_if 6 (;@1;)
                  local.get 2
                  local.get 0
                  i32.add
                  i32.load8_u
                  local.set 1
                  br 1 (;@6;)
                end
                i32.const 0
                local.set 7
                local.get 6
                i32.const 0
                i32.store
                local.get 2
                i32.load offset=4
                local.set 1
                local.get 2
                i64.const 0
                i64.store
              end
              local.get 4
              local.get 1
              local.get 5
              call_indirect (type 2)
              i32.eqz
              br_if 0 (;@5;)
              br 3 (;@2;)
            end
          end
          local.get 2
          i32.load8_u offset=10
          local.tee 1
          i32.const 10
          local.get 1
          i32.const 10
          i32.gt_u
          select
          local.set 0
          local.get 2
          i32.load8_u offset=11
          local.tee 7
          local.get 1
          local.get 7
          local.get 1
          i32.gt_u
          select
          local.set 8
          loop  ;; label = @4
            local.get 8
            local.get 1
            i32.eq
            br_if 1 (;@3;)
            local.get 2
            local.get 1
            i32.const 1
            i32.add
            local.tee 7
            i32.store8 offset=10
            local.get 0
            local.get 1
            i32.eq
            br_if 3 (;@1;)
            local.get 2
            local.get 1
            i32.add
            local.set 6
            local.get 7
            local.set 1
            local.get 4
            local.get 6
            i32.load8_u
            local.get 5
            call_indirect (type 2)
            i32.eqz
            br_if 0 (;@4;)
            br 2 (;@2;)
          end
        end
        local.get 4
        i32.const 39
        local.get 5
        call_indirect (type 2)
        local.set 3
      end
      local.get 2
      i32.const 16
      i32.add
      global.set 0
      local.get 3
      return
    end
    local.get 0
    i32.const 10
    i32.const 1052052
    call 34
    unreachable)
  (func (;62;) (type 2) (param i32 i32) (result i32)
    (local i32 i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    local.get 0
    i32.store offset=12
    local.get 2
    local.get 1
    i32.load offset=20
    i32.const 1052068
    i32.const 15
    local.get 1
    i32.const 24
    i32.add
    i32.load
    i32.load offset=12
    call_indirect (type 1)
    i32.store8 offset=24
    local.get 2
    local.get 1
    i32.store offset=20
    local.get 2
    i32.const 0
    i32.store8 offset=25
    local.get 2
    i32.const 0
    i32.store offset=16
    local.get 2
    i32.const 16
    i32.add
    local.get 2
    i32.const 12
    i32.add
    i32.const 1052084
    call 55
    local.set 1
    local.get 2
    i32.load8_u offset=24
    local.set 0
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.load
        local.tee 3
        br_if 0 (;@2;)
        local.get 0
        i32.const 255
        i32.and
        i32.const 0
        i32.ne
        local.set 1
        br 1 (;@1;)
      end
      i32.const 1
      local.set 1
      local.get 0
      i32.const 255
      i32.and
      br_if 0 (;@1;)
      local.get 2
      i32.load offset=20
      local.set 0
      block  ;; label = @2
        local.get 3
        i32.const 1
        i32.ne
        br_if 0 (;@2;)
        local.get 2
        i32.load8_u offset=25
        i32.const 255
        i32.and
        i32.eqz
        br_if 0 (;@2;)
        local.get 0
        i32.load8_u offset=28
        i32.const 4
        i32.and
        br_if 0 (;@2;)
        i32.const 1
        local.set 1
        local.get 0
        i32.load offset=20
        i32.const 1049707
        i32.const 1
        local.get 0
        i32.const 24
        i32.add
        i32.load
        i32.load offset=12
        call_indirect (type 1)
        br_if 1 (;@1;)
      end
      local.get 0
      i32.load offset=20
      i32.const 1049396
      i32.const 1
      local.get 0
      i32.const 24
      i32.add
      i32.load
      i32.load offset=12
      call_indirect (type 1)
      local.set 1
    end
    local.get 2
    i32.const 32
    i32.add
    global.set 0
    local.get 1)
  (func (;63;) (type 2) (param i32 i32) (result i32)
    local.get 1
    i32.const 1050016
    i32.const 2
    call 41)
  (func (;64;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32)
    global.get 0
    i32.const 128
    i32.sub
    local.tee 2
    global.set 0
    local.get 0
    i32.load
    local.set 0
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.load offset=28
            local.tee 3
            i32.const 16
            i32.and
            br_if 0 (;@4;)
            local.get 3
            i32.const 32
            i32.and
            br_if 1 (;@3;)
            local.get 0
            i64.load8_u
            local.get 1
            call 36
            local.set 0
            br 3 (;@1;)
          end
          local.get 0
          i32.load8_u
          local.set 3
          i32.const 0
          local.set 0
          loop  ;; label = @4
            local.get 2
            local.get 0
            i32.add
            i32.const 127
            i32.add
            i32.const 48
            i32.const 87
            local.get 3
            i32.const 15
            i32.and
            local.tee 4
            i32.const 10
            i32.lt_u
            select
            local.get 4
            i32.add
            i32.store8
            local.get 0
            i32.const -1
            i32.add
            local.set 0
            local.get 3
            i32.const 255
            i32.and
            local.tee 4
            i32.const 4
            i32.shr_u
            local.set 3
            local.get 4
            i32.const 16
            i32.ge_u
            br_if 0 (;@4;)
            br 2 (;@2;)
          end
        end
        local.get 0
        i32.load8_u
        local.set 3
        i32.const 0
        local.set 0
        loop  ;; label = @3
          local.get 2
          local.get 0
          i32.add
          i32.const 127
          i32.add
          i32.const 48
          i32.const 55
          local.get 3
          i32.const 15
          i32.and
          local.tee 4
          i32.const 10
          i32.lt_u
          select
          local.get 4
          i32.add
          i32.store8
          local.get 0
          i32.const -1
          i32.add
          local.set 0
          local.get 3
          i32.const 255
          i32.and
          local.tee 4
          i32.const 4
          i32.shr_u
          local.set 3
          local.get 4
          i32.const 16
          i32.ge_u
          br_if 0 (;@3;)
        end
        block  ;; label = @3
          local.get 0
          i32.const 128
          i32.add
          local.tee 3
          i32.const 129
          i32.lt_u
          br_if 0 (;@3;)
          local.get 3
          i32.const 128
          i32.const 1049740
          call 39
          unreachable
        end
        local.get 1
        i32.const 1049756
        i32.const 2
        local.get 2
        local.get 0
        i32.add
        i32.const 128
        i32.add
        i32.const 0
        local.get 0
        i32.sub
        call 37
        local.set 0
        br 1 (;@1;)
      end
      block  ;; label = @2
        local.get 0
        i32.const 128
        i32.add
        local.tee 3
        i32.const 129
        i32.lt_u
        br_if 0 (;@2;)
        local.get 3
        i32.const 128
        i32.const 1049740
        call 39
        unreachable
      end
      local.get 1
      i32.const 1049756
      i32.const 2
      local.get 2
      local.get 0
      i32.add
      i32.const 128
      i32.add
      i32.const 0
      local.get 0
      i32.sub
      call 37
      local.set 0
    end
    local.get 2
    i32.const 128
    i32.add
    global.set 0
    local.get 0)
  (func (;65;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i32 i64 i32)
    global.get 0
    i32.const 32
    i32.sub
    local.tee 2
    global.set 0
    local.get 0
    i32.load
    local.tee 0
    i32.load offset=8
    local.set 3
    local.get 0
    i32.load
    local.set 4
    i32.const 1
    local.set 5
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.load offset=20
        local.tee 6
        i32.const 34
        local.get 1
        i32.const 24
        i32.add
        i32.load
        local.tee 7
        i32.load offset=16
        local.tee 8
        call_indirect (type 2)
        br_if 0 (;@2;)
        block  ;; label = @3
          block  ;; label = @4
            local.get 3
            br_if 0 (;@4;)
            i32.const 0
            local.set 1
            i32.const 0
            local.set 3
            br 1 (;@3;)
          end
          local.get 4
          local.get 3
          i32.add
          local.set 9
          i32.const 0
          local.set 1
          local.get 4
          local.set 10
          i32.const 0
          local.set 11
          block  ;; label = @4
            block  ;; label = @5
              loop  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 10
                    local.tee 12
                    i32.load8_s
                    local.tee 0
                    i32.const -1
                    i32.le_s
                    br_if 0 (;@8;)
                    local.get 12
                    i32.const 1
                    i32.add
                    local.set 10
                    local.get 0
                    i32.const 255
                    i32.and
                    local.set 13
                    br 1 (;@7;)
                  end
                  local.get 12
                  i32.load8_u offset=1
                  i32.const 63
                  i32.and
                  local.set 14
                  local.get 0
                  i32.const 31
                  i32.and
                  local.set 15
                  block  ;; label = @8
                    local.get 0
                    i32.const -33
                    i32.gt_u
                    br_if 0 (;@8;)
                    local.get 15
                    i32.const 6
                    i32.shl
                    local.get 14
                    i32.or
                    local.set 13
                    local.get 12
                    i32.const 2
                    i32.add
                    local.set 10
                    br 1 (;@7;)
                  end
                  local.get 14
                  i32.const 6
                  i32.shl
                  local.get 12
                  i32.load8_u offset=2
                  i32.const 63
                  i32.and
                  i32.or
                  local.set 14
                  local.get 12
                  i32.const 3
                  i32.add
                  local.set 10
                  block  ;; label = @8
                    local.get 0
                    i32.const -16
                    i32.ge_u
                    br_if 0 (;@8;)
                    local.get 14
                    local.get 15
                    i32.const 12
                    i32.shl
                    i32.or
                    local.set 13
                    br 1 (;@7;)
                  end
                  local.get 14
                  i32.const 6
                  i32.shl
                  local.get 10
                  i32.load8_u
                  i32.const 63
                  i32.and
                  i32.or
                  local.get 15
                  i32.const 18
                  i32.shl
                  i32.const 1835008
                  i32.and
                  i32.or
                  local.tee 13
                  i32.const 1114112
                  i32.eq
                  br_if 3 (;@4;)
                  local.get 12
                  i32.const 4
                  i32.add
                  local.set 10
                end
                local.get 2
                local.get 13
                i32.const 65537
                call 58
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 2
                    i32.load8_u
                    i32.const 128
                    i32.eq
                    br_if 0 (;@8;)
                    local.get 2
                    i32.load8_u offset=11
                    local.get 2
                    i32.load8_u offset=10
                    i32.sub
                    i32.const 255
                    i32.and
                    i32.const 1
                    i32.eq
                    br_if 0 (;@8;)
                    local.get 11
                    local.get 1
                    i32.lt_u
                    br_if 3 (;@5;)
                    block  ;; label = @9
                      local.get 1
                      i32.eqz
                      br_if 0 (;@9;)
                      block  ;; label = @10
                        local.get 1
                        local.get 3
                        i32.lt_u
                        br_if 0 (;@10;)
                        local.get 1
                        local.get 3
                        i32.eq
                        br_if 1 (;@9;)
                        br 5 (;@5;)
                      end
                      local.get 4
                      local.get 1
                      i32.add
                      i32.load8_s
                      i32.const -64
                      i32.lt_s
                      br_if 4 (;@5;)
                    end
                    block  ;; label = @9
                      local.get 11
                      i32.eqz
                      br_if 0 (;@9;)
                      block  ;; label = @10
                        local.get 11
                        local.get 3
                        i32.lt_u
                        br_if 0 (;@10;)
                        local.get 11
                        local.get 3
                        i32.eq
                        br_if 1 (;@9;)
                        br 5 (;@5;)
                      end
                      local.get 4
                      local.get 11
                      i32.add
                      i32.load8_s
                      i32.const -65
                      i32.le_s
                      br_if 4 (;@5;)
                    end
                    block  ;; label = @9
                      block  ;; label = @10
                        local.get 6
                        local.get 4
                        local.get 1
                        i32.add
                        local.get 11
                        local.get 1
                        i32.sub
                        local.get 7
                        i32.load offset=12
                        call_indirect (type 1)
                        br_if 0 (;@10;)
                        local.get 2
                        i32.const 16
                        i32.add
                        i32.const 8
                        i32.add
                        local.tee 15
                        local.get 2
                        i32.const 8
                        i32.add
                        i32.load
                        i32.store
                        local.get 2
                        local.get 2
                        i64.load
                        local.tee 16
                        i64.store offset=16
                        block  ;; label = @11
                          local.get 16
                          i32.wrap_i64
                          i32.const 255
                          i32.and
                          i32.const 128
                          i32.ne
                          br_if 0 (;@11;)
                          i32.const 128
                          local.set 14
                          loop  ;; label = @12
                            block  ;; label = @13
                              block  ;; label = @14
                                local.get 14
                                i32.const 255
                                i32.and
                                i32.const 128
                                i32.eq
                                br_if 0 (;@14;)
                                local.get 2
                                i32.load8_u offset=26
                                local.tee 0
                                local.get 2
                                i32.load8_u offset=27
                                i32.ge_u
                                br_if 5 (;@9;)
                                local.get 2
                                local.get 0
                                i32.const 1
                                i32.add
                                i32.store8 offset=26
                                local.get 0
                                i32.const 10
                                i32.ge_u
                                br_if 7 (;@7;)
                                local.get 2
                                i32.const 16
                                i32.add
                                local.get 0
                                i32.add
                                i32.load8_u
                                local.set 1
                                br 1 (;@13;)
                              end
                              i32.const 0
                              local.set 14
                              local.get 15
                              i32.const 0
                              i32.store
                              local.get 2
                              i32.load offset=20
                              local.set 1
                              local.get 2
                              i64.const 0
                              i64.store offset=16
                            end
                            local.get 6
                            local.get 1
                            local.get 8
                            call_indirect (type 2)
                            i32.eqz
                            br_if 0 (;@12;)
                            br 2 (;@10;)
                          end
                        end
                        local.get 2
                        i32.load8_u offset=26
                        local.tee 1
                        i32.const 10
                        local.get 1
                        i32.const 10
                        i32.gt_u
                        select
                        local.set 0
                        local.get 2
                        i32.load8_u offset=27
                        local.tee 14
                        local.get 1
                        local.get 14
                        local.get 1
                        i32.gt_u
                        select
                        local.set 17
                        loop  ;; label = @11
                          local.get 17
                          local.get 1
                          i32.eq
                          br_if 2 (;@9;)
                          local.get 2
                          local.get 1
                          i32.const 1
                          i32.add
                          local.tee 14
                          i32.store8 offset=26
                          local.get 0
                          local.get 1
                          i32.eq
                          br_if 4 (;@7;)
                          local.get 2
                          i32.const 16
                          i32.add
                          local.get 1
                          i32.add
                          local.set 15
                          local.get 14
                          local.set 1
                          local.get 6
                          local.get 15
                          i32.load8_u
                          local.get 8
                          call_indirect (type 2)
                          i32.eqz
                          br_if 0 (;@11;)
                        end
                      end
                      i32.const 1
                      local.set 5
                      br 7 (;@2;)
                    end
                    i32.const 1
                    local.set 1
                    block  ;; label = @9
                      local.get 13
                      i32.const 128
                      i32.lt_u
                      br_if 0 (;@9;)
                      i32.const 2
                      local.set 1
                      local.get 13
                      i32.const 2048
                      i32.lt_u
                      br_if 0 (;@9;)
                      i32.const 3
                      i32.const 4
                      local.get 13
                      i32.const 65536
                      i32.lt_u
                      select
                      local.set 1
                    end
                    local.get 1
                    local.get 11
                    i32.add
                    local.set 1
                  end
                  local.get 11
                  local.get 12
                  i32.sub
                  local.get 10
                  i32.add
                  local.set 11
                  local.get 10
                  local.get 9
                  i32.ne
                  br_if 1 (;@6;)
                  br 3 (;@4;)
                end
              end
              local.get 0
              i32.const 10
              i32.const 1052052
              call 34
              unreachable
            end
            local.get 4
            local.get 3
            local.get 1
            local.get 11
            i32.const 1050000
            call 56
            unreachable
          end
          block  ;; label = @4
            local.get 1
            br_if 0 (;@4;)
            i32.const 0
            local.set 1
            br 1 (;@3;)
          end
          block  ;; label = @4
            local.get 3
            local.get 1
            i32.gt_u
            br_if 0 (;@4;)
            local.get 3
            local.get 1
            i32.ne
            br_if 3 (;@1;)
            local.get 3
            local.get 1
            i32.sub
            local.set 0
            local.get 3
            local.set 1
            local.get 0
            local.set 3
            br 1 (;@3;)
          end
          local.get 4
          local.get 1
          i32.add
          i32.load8_s
          i32.const -65
          i32.le_s
          br_if 2 (;@1;)
          local.get 3
          local.get 1
          i32.sub
          local.set 3
        end
        local.get 6
        local.get 4
        local.get 1
        i32.add
        local.get 3
        local.get 7
        i32.load offset=12
        call_indirect (type 1)
        br_if 0 (;@2;)
        local.get 6
        i32.const 34
        local.get 8
        call_indirect (type 2)
        local.set 5
      end
      local.get 2
      i32.const 32
      i32.add
      global.set 0
      local.get 5
      return
    end
    local.get 4
    local.get 3
    local.get 1
    local.get 3
    i32.const 1049984
    call 56
    unreachable)
  (func (;66;) (type 21) (param i32 i32 i32 i32)
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          local.get 1
          i32.eqz
          br_if 0 (;@3;)
          local.get 2
          i32.const -1
          i32.le_s
          br_if 1 (;@2;)
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                local.get 3
                i32.load offset=4
                i32.eqz
                br_if 0 (;@6;)
                block  ;; label = @7
                  local.get 3
                  i32.const 8
                  i32.add
                  i32.load
                  br_if 0 (;@7;)
                  i32.const 0
                  i32.load8_u offset=1053220
                  drop
                  br 2 (;@5;)
                end
                local.get 3
                i32.load
                local.get 2
                call 26
                local.set 1
                br 2 (;@4;)
              end
              i32.const 0
              i32.load8_u offset=1053220
              drop
            end
            local.get 2
            call 4
            local.set 1
          end
          block  ;; label = @4
            local.get 1
            i32.eqz
            br_if 0 (;@4;)
            local.get 0
            local.get 1
            i32.store offset=4
            local.get 0
            i32.const 8
            i32.add
            local.get 2
            i32.store
            i32.const 0
            local.set 1
            br 3 (;@1;)
          end
          i32.const 1
          local.set 1
          local.get 0
          i32.const 1
          i32.store offset=4
          local.get 0
          i32.const 8
          i32.add
          local.get 2
          i32.store
          br 2 (;@1;)
        end
        local.get 0
        i32.const 0
        i32.store offset=4
        local.get 0
        i32.const 8
        i32.add
        local.get 2
        i32.store
        i32.const 1
        local.set 1
        br 1 (;@1;)
      end
      local.get 0
      i32.const 0
      i32.store offset=4
      i32.const 1
      local.set 1
    end
    local.get 0
    local.get 1
    i32.store)
  (func (;67;) (type 18)
    unreachable
    unreachable)
  (func (;68;) (type 12) (param i32 i32)
    (local i32 i32 i32 i32)
    i32.const 31
    local.set 2
    block  ;; label = @1
      local.get 1
      i32.const 16777215
      i32.gt_u
      br_if 0 (;@1;)
      local.get 1
      i32.const 6
      local.get 1
      i32.const 8
      i32.shr_u
      i32.clz
      local.tee 2
      i32.sub
      i32.shr_u
      i32.const 1
      i32.and
      local.get 2
      i32.const 1
      i32.shl
      i32.sub
      i32.const 62
      i32.add
      local.set 2
    end
    local.get 0
    i64.const 0
    i64.store offset=16 align=4
    local.get 0
    local.get 2
    i32.store offset=28
    local.get 2
    i32.const 2
    i32.shl
    i32.const 1053232
    i32.add
    local.set 3
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              i32.const 0
              i32.load offset=1053644
              local.tee 4
              i32.const 1
              local.get 2
              i32.shl
              local.tee 5
              i32.and
              i32.eqz
              br_if 0 (;@5;)
              local.get 3
              i32.load
              local.tee 4
              i32.load offset=4
              i32.const -8
              i32.and
              local.get 1
              i32.ne
              br_if 1 (;@4;)
              local.get 4
              local.set 2
              br 2 (;@3;)
            end
            i32.const 0
            local.get 4
            local.get 5
            i32.or
            i32.store offset=1053644
            local.get 3
            local.get 0
            i32.store
            local.get 0
            local.get 3
            i32.store offset=24
            br 3 (;@1;)
          end
          local.get 1
          i32.const 0
          i32.const 25
          local.get 2
          i32.const 1
          i32.shr_u
          i32.sub
          i32.const 31
          i32.and
          local.get 2
          i32.const 31
          i32.eq
          select
          i32.shl
          local.set 3
          loop  ;; label = @4
            local.get 4
            local.get 3
            i32.const 29
            i32.shr_u
            i32.const 4
            i32.and
            i32.add
            i32.const 16
            i32.add
            local.tee 5
            i32.load
            local.tee 2
            i32.eqz
            br_if 2 (;@2;)
            local.get 3
            i32.const 1
            i32.shl
            local.set 3
            local.get 2
            local.set 4
            local.get 2
            i32.load offset=4
            i32.const -8
            i32.and
            local.get 1
            i32.ne
            br_if 0 (;@4;)
          end
        end
        local.get 2
        i32.load offset=8
        local.tee 3
        local.get 0
        i32.store offset=12
        local.get 2
        local.get 0
        i32.store offset=8
        local.get 0
        i32.const 0
        i32.store offset=24
        local.get 0
        local.get 2
        i32.store offset=12
        local.get 0
        local.get 3
        i32.store offset=8
        return
      end
      local.get 5
      local.get 0
      i32.store
      local.get 0
      local.get 4
      i32.store offset=24
    end
    local.get 0
    local.get 0
    i32.store offset=12
    local.get 0
    local.get 0
    i32.store offset=8)
  (func (;69;) (type 0) (param i32)
    local.get 0
    call 70
    unreachable)
  (func (;70;) (type 0) (param i32)
    (local i32 i32)
    local.get 0
    i32.load
    local.tee 1
    i32.const 12
    i32.add
    i32.load
    local.set 2
    block  ;; label = @1
      block  ;; label = @2
        local.get 1
        i32.load offset=4
        br_table 0 (;@2;) 0 (;@2;) 1 (;@1;)
      end
      local.get 2
      br_if 0 (;@1;)
      local.get 0
      i32.load offset=4
      i32.load8_u offset=16
      call 71
      unreachable
    end
    local.get 0
    i32.load offset=4
    i32.load8_u offset=16
    call 71
    unreachable)
  (func (;71;) (type 0) (param i32)
    (local i32)
    i32.const 0
    i32.const 0
    i32.load offset=1053228
    local.tee 1
    i32.const 1
    i32.add
    i32.store offset=1053228
    block  ;; label = @1
      local.get 1
      i32.const 0
      i32.lt_s
      br_if 0 (;@1;)
      i32.const 0
      i32.load8_u offset=1053688
      i32.const 1
      i32.and
      br_if 0 (;@1;)
      i32.const 0
      i32.const 1
      i32.store8 offset=1053688
      i32.const 0
      i32.const 0
      i32.load offset=1053684
      i32.const 1
      i32.add
      i32.store offset=1053684
      i32.const 0
      i32.load offset=1053224
      i32.const -1
      i32.le_s
      br_if 0 (;@1;)
      i32.const 0
      i32.const 0
      i32.store8 offset=1053688
      local.get 0
      i32.eqz
      br_if 0 (;@1;)
      call 67
      unreachable
    end
    unreachable
    unreachable)
  (func (;72;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 2
    global.set 0
    local.get 2
    local.get 0
    i32.load
    i32.store offset=12
    local.get 1
    i32.const 1053196
    i32.const 7
    local.get 2
    i32.const 12
    i32.add
    i32.const 1053204
    call 11
    local.set 0
    local.get 2
    i32.const 16
    i32.add
    global.set 0
    local.get 0)
  (func (;73;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32)
    global.get 0
    i32.const 64
    i32.sub
    local.tee 2
    global.set 0
    local.get 0
    i32.load
    local.set 3
    i32.const 0
    local.set 0
    local.get 1
    i32.load offset=20
    i32.const 1049436
    i32.const 1
    local.get 1
    i32.const 24
    i32.add
    i32.load
    i32.load offset=12
    call_indirect (type 1)
    local.set 4
    i32.const 1
    local.set 5
    loop  ;; label = @1
      local.get 2
      local.get 3
      local.get 0
      i32.add
      i32.store offset=4
      local.get 0
      i32.const 1
      i32.add
      local.set 0
      local.get 4
      i32.const 1
      i32.and
      local.set 6
      i32.const 1
      local.set 4
      block  ;; label = @2
        local.get 6
        br_if 0 (;@2;)
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  local.get 1
                  i32.load offset=28
                  local.tee 6
                  i32.const 4
                  i32.and
                  br_if 0 (;@7;)
                  local.get 5
                  i32.const 1
                  i32.and
                  i32.eqz
                  br_if 1 (;@6;)
                  br 4 (;@3;)
                end
                local.get 5
                i32.const 1
                i32.and
                br_if 1 (;@5;)
                local.get 1
                i32.load offset=24
                local.set 5
                local.get 1
                i32.load offset=20
                local.set 7
                br 2 (;@4;)
              end
              i32.const 1
              local.set 4
              local.get 1
              i32.load offset=20
              i32.const 1049700
              i32.const 2
              local.get 1
              i32.load offset=24
              i32.load offset=12
              call_indirect (type 1)
              i32.eqz
              br_if 2 (;@3;)
              br 3 (;@2;)
            end
            i32.const 1
            local.set 4
            local.get 1
            i32.load offset=20
            local.tee 7
            i32.const 1049708
            i32.const 1
            local.get 1
            i32.load offset=24
            local.tee 5
            i32.load offset=12
            call_indirect (type 1)
            br_if 2 (;@2;)
          end
          local.get 2
          i32.const 1
          i32.store8 offset=23
          local.get 2
          local.get 5
          i32.store offset=12
          local.get 2
          local.get 7
          i32.store offset=8
          local.get 2
          local.get 6
          i32.store offset=52
          local.get 2
          i32.const 1049672
          i32.store offset=48
          local.get 2
          local.get 1
          i32.load8_u offset=32
          i32.store8 offset=56
          local.get 2
          local.get 1
          i32.load offset=16
          i32.store offset=40
          local.get 2
          local.get 1
          i64.load offset=8 align=4
          i64.store offset=32
          local.get 2
          local.get 1
          i64.load align=4
          i64.store offset=24
          local.get 2
          local.get 2
          i32.const 23
          i32.add
          i32.store offset=16
          local.get 2
          local.get 2
          i32.const 8
          i32.add
          i32.store offset=44
          block  ;; label = @4
            local.get 2
            i32.const 4
            i32.add
            local.get 2
            i32.const 24
            i32.add
            call 64
            br_if 0 (;@4;)
            local.get 2
            i32.load offset=44
            i32.const 1049702
            i32.const 2
            local.get 2
            i32.load offset=48
            i32.load offset=12
            call_indirect (type 1)
            local.set 4
            br 2 (;@2;)
          end
          i32.const 1
          local.set 4
          br 1 (;@2;)
        end
        local.get 2
        i32.const 4
        i32.add
        local.get 1
        call 64
        local.set 4
      end
      i32.const 0
      local.set 5
      local.get 0
      i32.const 32
      i32.ne
      br_if 0 (;@1;)
    end
    i32.const 1
    local.set 0
    block  ;; label = @1
      local.get 4
      br_if 0 (;@1;)
      local.get 1
      i32.load offset=20
      i32.const 1049709
      i32.const 1
      local.get 1
      i32.load offset=24
      i32.load offset=12
      call_indirect (type 1)
      local.set 0
    end
    local.get 2
    i32.const 64
    i32.add
    global.set 0
    local.get 0)
  (func (;74;) (type 2) (param i32 i32) (result i32)
    (local i32 i32 i32)
    global.get 0
    i32.const 128
    i32.sub
    local.tee 2
    global.set 0
    local.get 0
    i32.load
    local.set 0
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            local.get 1
            i32.load offset=28
            local.tee 3
            i32.const 16
            i32.and
            br_if 0 (;@4;)
            local.get 3
            i32.const 32
            i32.and
            br_if 1 (;@3;)
            local.get 0
            i64.load32_u
            local.get 1
            call 36
            local.set 0
            br 3 (;@1;)
          end
          local.get 0
          i32.load
          local.set 0
          i32.const 0
          local.set 3
          loop  ;; label = @4
            local.get 2
            local.get 3
            i32.add
            i32.const 127
            i32.add
            i32.const 48
            i32.const 87
            local.get 0
            i32.const 15
            i32.and
            local.tee 4
            i32.const 10
            i32.lt_u
            select
            local.get 4
            i32.add
            i32.store8
            local.get 3
            i32.const -1
            i32.add
            local.set 3
            local.get 0
            i32.const 16
            i32.lt_u
            local.set 4
            local.get 0
            i32.const 4
            i32.shr_u
            local.set 0
            local.get 4
            i32.eqz
            br_if 0 (;@4;)
            br 2 (;@2;)
          end
        end
        local.get 0
        i32.load
        local.set 0
        i32.const 0
        local.set 3
        loop  ;; label = @3
          local.get 2
          local.get 3
          i32.add
          i32.const 127
          i32.add
          i32.const 48
          i32.const 55
          local.get 0
          i32.const 15
          i32.and
          local.tee 4
          i32.const 10
          i32.lt_u
          select
          local.get 4
          i32.add
          i32.store8
          local.get 3
          i32.const -1
          i32.add
          local.set 3
          local.get 0
          i32.const 16
          i32.lt_u
          local.set 4
          local.get 0
          i32.const 4
          i32.shr_u
          local.set 0
          local.get 4
          i32.eqz
          br_if 0 (;@3;)
        end
        block  ;; label = @3
          local.get 3
          i32.const 128
          i32.add
          local.tee 0
          i32.const 129
          i32.lt_u
          br_if 0 (;@3;)
          local.get 0
          i32.const 128
          i32.const 1049740
          call 39
          unreachable
        end
        local.get 1
        i32.const 1049756
        i32.const 2
        local.get 2
        local.get 3
        i32.add
        i32.const 128
        i32.add
        i32.const 0
        local.get 3
        i32.sub
        call 37
        local.set 0
        br 1 (;@1;)
      end
      block  ;; label = @2
        local.get 3
        i32.const 128
        i32.add
        local.tee 0
        i32.const 129
        i32.lt_u
        br_if 0 (;@2;)
        local.get 0
        i32.const 128
        i32.const 1049740
        call 39
        unreachable
      end
      local.get 1
      i32.const 1049756
      i32.const 2
      local.get 2
      local.get 3
      i32.add
      i32.const 128
      i32.add
      i32.const 0
      local.get 3
      i32.sub
      call 37
      local.set 0
    end
    local.get 2
    i32.const 128
    i32.add
    global.set 0
    local.get 0)
  (func (;75;) (type 7) (param i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 1
    global.set 0
    local.get 1
    i32.const 8
    i32.add
    local.get 0
    call 17
    local.get 1
    i32.load offset=8
    local.set 0
    local.get 1
    i32.const 16
    i32.add
    global.set 0
    local.get 0)
  (func (;76;) (type 12) (param i32 i32)
    block  ;; label = @1
      local.get 1
      i32.eqz
      br_if 0 (;@1;)
      local.get 0
      call 6
    end)
  (func (;77;) (type 2) (param i32 i32) (result i32)
    (local i32)
    global.get 0
    i32.const 16
    i32.sub
    local.tee 2
    global.set 0
    block  ;; label = @1
      block  ;; label = @2
        block  ;; label = @3
          block  ;; label = @4
            block  ;; label = @5
              block  ;; label = @6
                block  ;; label = @7
                  block  ;; label = @8
                    local.get 0
                    i32.load8_u
                    br_table 0 (;@8;) 1 (;@7;) 2 (;@6;) 3 (;@5;) 4 (;@4;) 5 (;@3;) 6 (;@2;) 0 (;@8;)
                  end
                  local.get 2
                  local.get 0
                  i32.const 4
                  i32.add
                  i32.store offset=4
                  local.get 1
                  i32.const 1053048
                  i32.const 5
                  local.get 2
                  i32.const 4
                  i32.add
                  i32.const 1053056
                  call 11
                  local.set 1
                  br 6 (;@1;)
                end
                local.get 1
                i32.load offset=20
                i32.const 1053072
                i32.const 12
                local.get 1
                i32.const 24
                i32.add
                i32.load
                i32.load offset=12
                call_indirect (type 1)
                local.set 1
                br 5 (;@1;)
              end
              local.get 2
              local.get 0
              i32.const 4
              i32.add
              i32.store offset=8
              local.get 1
              i32.const 1053084
              i32.const 17
              local.get 2
              i32.const 8
              i32.add
              i32.const 1053104
              call 11
              local.set 1
              br 4 (;@1;)
            end
            local.get 2
            local.get 0
            i32.const 1
            i32.add
            i32.store offset=12
            local.get 1
            i32.const 1053120
            i32.const 10
            local.get 2
            i32.const 12
            i32.add
            i32.const 1053132
            call 11
            local.set 1
            br 3 (;@1;)
          end
          local.get 1
          i32.load offset=20
          i32.const 1053148
          i32.const 5
          local.get 1
          i32.const 24
          i32.add
          i32.load
          i32.load offset=12
          call_indirect (type 1)
          local.set 1
          br 2 (;@1;)
        end
        local.get 1
        i32.load offset=20
        i32.const 1053153
        i32.const 4
        local.get 1
        i32.const 24
        i32.add
        i32.load
        i32.load offset=12
        call_indirect (type 1)
        local.set 1
        br 1 (;@1;)
      end
      local.get 1
      i32.load offset=20
      i32.const 1053157
      i32.const 13
      local.get 1
      i32.const 24
      i32.add
      i32.load
      i32.load offset=12
      call_indirect (type 1)
      local.set 1
    end
    local.get 2
    i32.const 16
    i32.add
    global.set 0
    local.get 1)
  (func (;78;) (type 0) (param i32))
  (func (;79;) (type 1) (param i32 i32 i32) (result i32)
    (local i32 i32 i32 i32 i32 i32 i32 i32)
    block  ;; label = @1
      block  ;; label = @2
        local.get 2
        i32.const 15
        i32.gt_u
        br_if 0 (;@2;)
        local.get 0
        local.set 3
        br 1 (;@1;)
      end
      local.get 0
      i32.const 0
      local.get 0
      i32.sub
      i32.const 3
      i32.and
      local.tee 4
      i32.add
      local.set 5
      block  ;; label = @2
        local.get 4
        i32.eqz
        br_if 0 (;@2;)
        local.get 0
        local.set 3
        local.get 1
        local.set 6
        loop  ;; label = @3
          local.get 3
          local.get 6
          i32.load8_u
          i32.store8
          local.get 6
          i32.const 1
          i32.add
          local.set 6
          local.get 3
          i32.const 1
          i32.add
          local.tee 3
          local.get 5
          i32.lt_u
          br_if 0 (;@3;)
        end
      end
      local.get 5
      local.get 2
      local.get 4
      i32.sub
      local.tee 7
      i32.const -4
      i32.and
      local.tee 8
      i32.add
      local.set 3
      block  ;; label = @2
        block  ;; label = @3
          local.get 1
          local.get 4
          i32.add
          local.tee 9
          i32.const 3
          i32.and
          i32.eqz
          br_if 0 (;@3;)
          local.get 8
          i32.const 1
          i32.lt_s
          br_if 1 (;@2;)
          local.get 9
          i32.const 3
          i32.shl
          local.tee 6
          i32.const 24
          i32.and
          local.set 2
          local.get 9
          i32.const -4
          i32.and
          local.tee 10
          i32.const 4
          i32.add
          local.set 1
          i32.const 0
          local.get 6
          i32.sub
          i32.const 24
          i32.and
          local.set 4
          local.get 10
          i32.load
          local.set 6
          loop  ;; label = @4
            local.get 5
            local.get 6
            local.get 2
            i32.shr_u
            local.get 1
            i32.load
            local.tee 6
            local.get 4
            i32.shl
            i32.or
            i32.store
            local.get 1
            i32.const 4
            i32.add
            local.set 1
            local.get 5
            i32.const 4
            i32.add
            local.tee 5
            local.get 3
            i32.lt_u
            br_if 0 (;@4;)
            br 2 (;@2;)
          end
        end
        local.get 8
        i32.const 1
        i32.lt_s
        br_if 0 (;@2;)
        local.get 9
        local.set 1
        loop  ;; label = @3
          local.get 5
          local.get 1
          i32.load
          i32.store
          local.get 1
          i32.const 4
          i32.add
          local.set 1
          local.get 5
          i32.const 4
          i32.add
          local.tee 5
          local.get 3
          i32.lt_u
          br_if 0 (;@3;)
        end
      end
      local.get 7
      i32.const 3
      i32.and
      local.set 2
      local.get 9
      local.get 8
      i32.add
      local.set 1
    end
    block  ;; label = @1
      local.get 2
      i32.eqz
      br_if 0 (;@1;)
      local.get 3
      local.get 2
      i32.add
      local.set 5
      loop  ;; label = @2
        local.get 3
        local.get 1
        i32.load8_u
        i32.store8
        local.get 1
        i32.const 1
        i32.add
        local.set 1
        local.get 3
        i32.const 1
        i32.add
        local.tee 3
        local.get 5
        i32.lt_u
        br_if 0 (;@2;)
      end
    end
    local.get 0)
  (func (;80;) (type 1) (param i32 i32 i32) (result i32)
    (local i32 i32 i32)
    i32.const 0
    local.set 3
    block  ;; label = @1
      local.get 2
      i32.eqz
      br_if 0 (;@1;)
      block  ;; label = @2
        loop  ;; label = @3
          local.get 0
          i32.load8_u
          local.tee 4
          local.get 1
          i32.load8_u
          local.tee 5
          i32.ne
          br_if 1 (;@2;)
          local.get 0
          i32.const 1
          i32.add
          local.set 0
          local.get 1
          i32.const 1
          i32.add
          local.set 1
          local.get 2
          i32.const -1
          i32.add
          local.tee 2
          i32.eqz
          br_if 2 (;@1;)
          br 0 (;@3;)
        end
      end
      local.get 4
      local.get 5
      i32.sub
      local.set 3
    end
    local.get 3)
  (func (;81;) (type 1) (param i32 i32 i32) (result i32)
    local.get 0
    local.get 1
    local.get 2
    call 79)
  (func (;82;) (type 1) (param i32 i32 i32) (result i32)
    local.get 0
    local.get 1
    local.get 2
    call 80)
  (table (;0;) 29 29 funcref)
  (memory (;0;) 17)
  (global (;0;) (mut i32) (i32.const 1048576))
  (global (;1;) i32 (i32.const 1053689))
  (global (;2;) i32 (i32.const 1053696))
  (export "memory" (memory 0))
  (export "init_guest" (func 19))
  (export "get_total_supply_guest" (func 20))
  (export "mint_to_guest" (func 21))
  (export "transfer_guest" (func 23))
  (export "get_balance_guest" (func 25))
  (export "alloc" (func 75))
  (export "dealloc" (func 76))
  (export "__data_end" (global 1))
  (export "__heap_base" (global 2))
  (elem (;0;) (i32.const 1) func 42 43 44 35 30 60 61 13 77 12 62 78 10 32 33 46 49 50 51 52 53 54 63 65 74 64 72 73)
  (data (;0;) (i32.const 1048576) "conversion from i32/home/sam.batschelet/projects/ava-labs/hypersdk/hypersdk/x/programs/rust/wasmlanche_sdk/src/state.rs\00\13\00\10\00d\00\00\00I\00\00\00$\00\00\00\08\00\00\00\10\00\00\00\04\00\00\00\09\00\00\00\0a\00\00\00\00\00\00\00\01\00\00\00\0b\00\00\00\0c\00\00\00\04\00\00\00\04\00\00\00\0d\00\00\00failed to store total supplyx/programs/rust/examples/token/src/lib.rs\00\00\00\d4\00\10\00)\00\00\00\17\00\00\00\0a\00\00\00WasmCoinfailed to store coin name\00\00\00\d4\00\10\00)\00\00\00\1d\00\00\00\0a\00\00\00WACKfailed to store symbol\00\00\d4\00\10\00)\00\00\00#\00\00\00\0a\00\00\00failed to get total supply\00\00\d4\00\10\00)\00\00\00.\00\00\00\0a\00\00\00\d4\00\10\00)\00\00\00;\00\00\008\00\00\00\00\00\00\00attempt to add with overflowfailed to store balance\00\d4\00\10\00)\00\00\00<\00\00\00\0a\00\00\00failed to update balance\d4\00\10\00)\00\00\00J\00\00\00\0a\00\00\00\d4\00\10\00)\00\00\00[\00\00\00\0e\00\00\00\00\00\00\00attempt to subtract with overflow\00\00\00\d4\00\10\00)\00\00\00]\00\00\00\0a\00\00\00\d4\00\10\00)\00\00\00c\00\00\00\0e\00\00\00\d4\00\10\00)\00\00\00e\00\00\00\0a\00\00\00invalid input\00\00\00\84\02\10\00\0d\00\00\00\d4\00\10\00)\00\00\00L\00\00\00\05\00\00\00sender and recipient must be different\00\00\ac\02\10\00&\00\00\00\d4\00\10\00)\00\00\00D\00\00\00\05\00\00\00library/alloc/src/raw_vec.rscapacity overflow\00\00\00\08\03\10\00\11\00\00\00\ec\02\10\00\1c\00\00\00\0c\02\00\00\05\00\00\00)library/core/src/fmt/mod.rs..\00\00P\03\10\00\02\00\00\00[\00\00\00\0e\00\00\00\00\00\00\00\01\00\00\00\0f\00\00\00index out of bounds: the len is  but the index is \00\00p\03\10\00 \00\00\00\90\03\10\00\12\00\00\00!=assertion failed: `(left  right)`\0a  left: ``,\0a right: ``\00\00\b6\03\10\00\19\00\00\00\cf\03\10\00\12\00\00\00\e1\03\10\00\0c\00\00\00\ed\03\10\00\01\00\00\00`: \00\b6\03\10\00\19\00\00\00\cf\03\10\00\12\00\00\00\e1\03\10\00\0c\00\00\00\10\04\10\00\03\00\00\00: \00\00L\11\10\00\00\00\00\004\04\10\00\02\00\00\00\10\00\00\00\0c\00\00\00\04\00\00\00\11\00\00\00\12\00\00\00\13\00\00\00    , ,\0a((\0a,\0a]library/core/src/fmt/num.rs\00\00\00n\04\10\00\1b\00\00\00i\00\00\00\14\00\00\000x00010203040506070809101112131415161718192021222324252627282930313233343536373839404142434445464748495051525354555657585960616263646566676869707172737475767778798081828384858687888990919293949596979899\00\00\10\00\00\00\04\00\00\00\04\00\00\00\14\00\00\00\15\00\00\00\16\00\00\005\03\10\00\1b\00\00\00\1b\09\00\00\16\00\00\005\03\10\00\1b\00\00\00\14\09\00\00\1e\00\00\00()range start index  out of range for slice of length \00\00\a2\05\10\00\12\00\00\00\b4\05\10\00\22\00\00\00range end index \e8\05\10\00\10\00\00\00\b4\05\10\00\22\00\00\00slice index starts at  but ends at \00\08\06\10\00\16\00\00\00\1e\06\10\00\0d\00\00\00[...]byte index  is not a char boundary; it is inside  (bytes ) of `A\06\10\00\0b\00\00\00L\06\10\00&\00\00\00r\06\10\00\08\00\00\00z\06\10\00\06\00\00\00\ed\03\10\00\01\00\00\00begin <= end ( <= ) when slicing `\00\00\a8\06\10\00\0e\00\00\00\b6\06\10\00\04\00\00\00\ba\06\10\00\10\00\00\00\ed\03\10\00\01\00\00\00 is out of bounds of `\00\00A\06\10\00\0b\00\00\00\ec\06\10\00\16\00\00\00\ed\03\10\00\01\00\00\00library/core/src/str/mod.rs\00\1c\07\10\00\1b\00\00\00\03\01\00\00\1d\00\00\00library/core/src/unicode/printable.rs\00\00\00H\07\10\00%\00\00\00\1a\00\00\006\00\00\00H\07\10\00%\00\00\00\0a\00\00\00\1c\00\00\00\00\06\01\01\03\01\04\02\05\07\07\02\08\08\09\02\0a\05\0b\02\0e\04\10\01\11\02\12\05\13\11\14\01\15\02\17\02\19\0d\1c\05\1d\08\1f\01$\01j\04k\02\af\03\b1\02\bc\02\cf\02\d1\02\d4\0c\d5\09\d6\02\d7\02\da\01\e0\05\e1\02\e7\04\e8\02\ee \f0\04\f8\02\fa\03\fb\01\0c';>NO\8f\9e\9e\9f{\8b\93\96\a2\b2\ba\86\b1\06\07\096=>V\f3\d0\d1\04\14\1867VW\7f\aa\ae\af\bd5\e0\12\87\89\8e\9e\04\0d\0e\11\12)14:EFIJNOde\5c\b6\b7\1b\1c\07\08\0a\0b\14\1769:\a8\a9\d8\d9\097\90\91\a8\07\0a;>fi\8f\92\11o_\bf\ee\efZb\f4\fc\ffST\9a\9b./'(U\9d\a0\a1\a3\a4\a7\a8\ad\ba\bc\c4\06\0b\0c\15\1d:?EQ\a6\a7\cc\cd\a0\07\19\1a\22%>?\e7\ec\ef\ff\c5\c6\04 #%&(38:HJLPSUVXZ\5c^`cefksx}\7f\8a\a4\aa\af\b0\c0\d0\ae\afno\be\93^\22{\05\03\04-\03f\03\01/.\80\82\1d\031\0f\1c\04$\09\1e\05+\05D\04\0e*\80\aa\06$\04$\04(\084\0bNC\817\09\16\0a\08\18;E9\03c\08\090\16\05!\03\1b\05\01@8\04K\05/\04\0a\07\09\07@ '\04\0c\096\03:\05\1a\07\04\0c\07PI73\0d3\07.\08\0a\81&RK+\08*\16\1a&\1c\14\17\09N\04$\09D\0d\19\07\0a\06H\08'\09u\0bB>*\06;\05\0a\06Q\06\01\05\10\03\05\80\8bb\1eH\08\0a\80\a6^\22E\0b\0a\06\0d\13:\06\0a6,\04\17\80\b9<dS\0cH\09\0aFE\1bH\08S\0dI\07\0a\80\f6F\0a\1d\03GI7\03\0e\08\0a\069\07\0a\816\19\07;\03\1cV\01\0f2\0d\83\9bfu\0b\80\c4\8aLc\0d\840\10\16\8f\aa\82G\a1\b9\829\07*\04\5c\06&\0aF\0a(\05\13\82\b0[eK\049\07\11@\05\0b\02\0e\97\f8\08\84\d6*\09\a2\e7\813\0f\01\1d\06\0e\04\08\81\8c\89\04k\05\0d\03\09\07\10\92`G\09t<\80\f6\0as\08p\15Fz\14\0c\14\0cW\09\19\80\87\81G\03\85B\0f\15\84P\1f\06\06\80\d5+\05>!\01p-\03\1a\04\02\81@\1f\11:\05\01\81\d0*\82\e6\80\f7)L\04\0a\04\02\83\11DL=\80\c2<\06\01\04U\05\1b4\02\81\0e,\04d\0cV\0a\80\ae8\1d\0d,\04\09\07\02\0e\06\80\9a\83\d8\04\11\03\0d\03w\04_\06\0c\04\01\0f\0c\048\08\0a\06(\08\22N\81T\0c\1d\03\09\076\08\0e\04\09\07\09\07\80\cb%\0a\84\06\00\01\03\05\05\06\06\02\07\06\08\07\09\11\0a\1c\0b\19\0c\1a\0d\10\0e\0c\0f\04\10\03\12\12\13\09\16\01\17\04\18\01\19\03\1a\07\1b\01\1c\02\1f\16 \03+\03-\0b.\010\031\022\01\a7\02\a9\02\aa\04\ab\08\fa\02\fb\05\fd\02\fe\03\ff\09\adxy\8b\8d\a20WX\8b\8c\90\1c\dd\0e\0fKL\fb\fc./?\5c]_\e2\84\8d\8e\91\92\a9\b1\ba\bb\c5\c6\c9\ca\de\e4\e5\ff\00\04\11\12)147:;=IJ]\84\8e\92\a9\b1\b4\ba\bb\c6\ca\ce\cf\e4\e5\00\04\0d\0e\11\12)14:;EFIJ^de\84\91\9b\9d\c9\ce\cf\0d\11):;EIW[\5c^_de\8d\91\a9\b4\ba\bb\c5\c9\df\e4\e5\f0\0d\11EIde\80\84\b2\bc\be\bf\d5\d7\f0\f1\83\85\8b\a4\a6\be\bf\c5\c7\cf\da\dbH\98\bd\cd\c6\ce\cfINOWY^_\89\8e\8f\b1\b6\b7\bf\c1\c6\c7\d7\11\16\17[\5c\f6\f7\fe\ff\80mq\de\df\0e\1fno\1c\1d_}~\ae\af\7f\bb\bc\16\17\1e\1fFGNOXZ\5c^~\7f\b5\c5\d4\d5\dc\f0\f1\f5rs\8ftu\96&./\a7\af\b7\bf\c7\cf\d7\df\9a@\97\980\8f\1f\d2\d4\ce\ffNOZ[\07\08\0f\10'/\ee\efno7=?BE\90\91Sgu\c8\c9\d0\d1\d8\d9\e7\fe\ff\00 _\22\82\df\04\82D\08\1b\04\06\11\81\ac\0e\80\ab\05\1f\09\81\1b\03\19\08\01\04/\044\04\07\03\01\07\06\07\11\0aP\0f\12\07U\07\03\04\1c\0a\09\03\08\03\07\03\02\03\03\03\0c\04\05\03\0b\06\01\0e\15\05N\07\1b\07W\07\02\06\17\0cP\04C\03-\03\01\04\11\06\0f\0c:\04\1d%_ m\04j%\80\c8\05\82\b0\03\1a\06\82\fd\03Y\07\16\09\18\09\14\0c\14\0cj\06\0a\06\1a\06Y\07+\05F\0a,\04\0c\04\01\031\0b,\04\1a\06\0b\03\80\ac\06\0a\06/1M\03\80\a4\08<\03\0f\03<\078\08+\05\82\ff\11\18\08/\11-\03!\0f!\0f\80\8c\04\82\97\19\0b\15\88\94\05/\05;\07\02\0e\18\09\80\be\22t\0c\80\d6\1a\0c\05\80\ff\05\80\df\0c\f2\9d\037\09\81\5c\14\80\b8\08\80\cb\05\0a\18;\03\0a\068\08F\08\0c\06t\0b\1e\03Z\04Y\09\80\83\18\1c\0a\16\09L\04\80\8a\06\ab\a4\0c\17\041\a1\04\81\da&\07\0c\05\05\80\a6\10\81\f5\07\01 *\06L\04\80\8d\04\80\be\03\1b\03\0f\0dlibrary/core/src/unicode/unicode_data.rs\0c\0d\10\00(\00\00\00P\00\00\00(\00\00\00\0c\0d\10\00(\00\00\00\5c\00\00\00\16\00\00\000123456789abcdeflibrary/core/src/escape.rs\00\00d\0d\10\00\1a\00\00\004\00\00\00\05\00\00\00\5cu{\00d\0d\10\00\1a\00\00\00b\00\00\00#\00\00\00TryFromIntError\00\10\00\00\00\04\00\00\00\04\00\00\00\17\00\00\00\00\03\00\00\83\04 \00\91\05`\00]\13\a0\00\12\17 \1f\0c `\1f\ef,\a0+*0 ,o\a6\e0,\02\a8`-\1e\fb`.\00\fe 6\9e\ff`6\fd\01\e16\01\0a!7$\0d\e17\ab\0ea9/\18\a190\1caH\f3\1e\a1L@4aP\f0j\a1QOo!R\9d\bc\a1R\00\cfaSe\d1\a1S\00\da!T\00\e0\e1U\ae\e2aW\ec\e4!Y\d0\e8\a1Y \00\eeY\f0\01\7fZ\00p\00\07\00-\01\01\01\02\01\02\01\01H\0b0\15\10\01e\07\02\06\02\02\01\04#\01\1e\1b[\0b:\09\09\01\18\04\01\09\01\03\01\05+\03<\08*\18\01 7\01\01\01\04\08\04\01\03\07\0a\02\1d\01:\01\01\01\02\04\08\01\09\01\0a\02\1a\01\02\029\01\04\02\04\02\02\03\03\01\1e\02\03\01\0b\029\01\04\05\01\02\04\01\14\02\16\06\01\01:\01\01\02\01\04\08\01\07\03\0a\02\1e\01;\01\01\01\0c\01\09\01(\01\03\017\01\01\03\05\03\01\04\07\02\0b\02\1d\01:\01\02\01\02\01\03\01\05\02\07\02\0b\02\1c\029\02\01\01\02\04\08\01\09\01\0a\02\1d\01H\01\04\01\02\03\01\01\08\01Q\01\02\07\0c\08b\01\02\09\0b\07I\02\1b\01\01\01\01\017\0e\01\05\01\02\05\0b\01$\09\01f\04\01\06\01\02\02\02\19\02\04\03\10\04\0d\01\02\02\06\01\0f\01\00\03\00\03\1d\02\1e\02\1e\02@\02\01\07\08\01\02\0b\09\01-\03\01\01u\02\22\01v\03\04\02\09\01\06\03\db\02\02\01:\01\01\07\01\01\01\01\02\08\06\0a\02\010\1f1\040\07\01\01\05\01(\09\0c\02 \04\02\02\01\038\01\01\02\03\01\01\03:\08\02\02\98\03\01\0d\01\07\04\01\06\01\03\02\c6@\00\01\c3!\00\03\8d\01` \00\06i\02\00\04\01\0a \02P\02\00\01\03\01\04\01\19\02\05\01\97\02\1a\12\0d\01&\08\19\0b.\030\01\02\04\02\02'\01C\06\02\02\02\02\0c\01\08\01/\013\01\01\03\02\02\05\02\01\01*\02\08\01\ee\01\02\01\04\01\00\01\00\10\10\10\00\02\00\01\e2\01\95\05\00\03\01\02\05\04(\03\04\01\a5\02\00\04\00\02P\03F\0b1\04{\016\0f)\01\02\02\0a\031\04\02\02\07\01=\03$\05\01\08>\01\0c\024\09\0a\04\02\01_\03\02\01\01\02\06\01\02\01\9d\01\03\08\15\029\02\01\01\01\01\16\01\0e\07\03\05\c3\08\02\03\01\01\17\01Q\01\02\06\01\01\02\01\01\02\01\02\eb\01\02\04\06\02\01\02\1b\02U\08\02\01\01\02j\01\01\01\02\06\01\01e\03\02\04\01\05\00\09\01\02\f5\01\0a\02\01\01\04\01\90\04\02\02\04\01 \0a(\06\02\04\08\01\09\06\02\03.\0d\01\02\00\07\01\06\01\01R\16\02\07\01\02\01\02z\06\03\01\01\02\01\07\01\01H\02\03\01\01\01\00\02\0b\024\05\05\01\01\01\00\01\06\0f\00\05;\07\00\01?\04Q\01\00\02\00.\02\17\00\01\01\03\04\05\08\08\02\07\1e\04\94\03\007\042\08\01\0e\01\16\05\01\0f\00\07\01\11\02\07\01\02\01\05d\01\a0\07\00\01=\04\00\04\00\07m\07\00`\80\f0\00called `Option::unwrap()` on a `None` value\00\00library/std/src/panicking.rsL\11\10\00\1c\00\00\00P\02\00\00\1e\00\00\00Other\00\00\00\0c\00\00\00\04\00\00\00\04\00\00\00\18\00\00\00InvalidBytesInvalidByteLength\00\00\00\0c\00\00\00\04\00\00\00\04\00\00\00\19\00\00\00InvalidTag\00\00\0c\00\00\00\04\00\00\00\04\00\00\00\1a\00\00\00WriteReadSerializationAddress\00\00\00\0c\00\00\00\04\00\00\00\04\00\00\00\1b\00\00\00Bytes32\00\0c\00\00\00\04\00\00\00\04\00\00\00\1c\00\00\00"))
