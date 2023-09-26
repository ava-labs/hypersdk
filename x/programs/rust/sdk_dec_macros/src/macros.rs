/// Asserts that first expression is greater than the second.
/// Returns true if the condition is met.
/// Does not panic in the case the condition is not met, simply returns false.
///
/// Requires that both expressions be comparable with `>`.
///
/// ## Uses
///
/// Assertions are always checked in both debug and release builds, and cannot be disabled.
#[macro_export]
macro_rules! assert_gt {
    ($left:expr, $right:expr) => {
        match (&$left, &$right) {
            (left_val, right_val) => {
                if !(*left_val > *right_val) {
                    // The reborrows below are intentional. Without them, the stack slot for the
                    // borrow is initialized even before the values are compared, leading to a
                    // noticeable slow down.
                    false
                } else {
                    true
                }
            }
        }
    };
}

/// Asserts that first expression is equal to the second.
/// Returns true if the condition is met.
/// Does not panic in the case the condition is not met, simply returns false.
///
/// Requires that both expressions be comparable with `==`.
///
/// ## Uses
///
/// Assertions are always checked in both debug and release builds, and cannot be disabled.
#[macro_export]
macro_rules! assert_eq {
    ($left:expr, $right:expr) => {
        match (&$left, &$right) {
            (left_val, right_val) => {
                if (*left_val == *right_val) {
                    // The reborrows below are intentional. Without them, the stack slot for the
                    // borrow is initialized even before the values are compared, leading to a
                    // noticeable slow down.
                    true
                } else {
                    false
                }
            }
        }
    };
}

/// Asserts that first expression is equal to the second.
/// Returns true if the condition is met.
/// Does not panic in the case the condition is not met, simply returns false.
///
/// Requires that both expressions be comparable with `==`.
///
/// ## Uses
///
/// Assertions are always checked in both debug and release builds, and cannot be disabled.
#[macro_export]
macro_rules! require {
    ($left:expr) => {
        match (&$left) {
            (left_val) => {
                if (*left_val == true) {
                    // The reborrows below are intentional. Without them, the stack slot for the
                    // borrow is initialized even before the values are compared, leading to a
                    // noticeable slow down.
                    true
                } else {
                    false
                }
            }
        }
    };
}
