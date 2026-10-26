file(MAKE_DIRECTORY "${OUTPUT_DIR}")
execute_process(COMMAND "${JAVAC}" -d "${OUTPUT_DIR}"
    "${SOURCE_DIR}/android/src/org/litterbox/CrashStore.java"
    "${SOURCE_DIR}/tests/CrashStoreTest.java" RESULT_VARIABLE compiled)
if(NOT compiled EQUAL 0)
    message(FATAL_ERROR "CrashStore oracle compilation failed: ${compiled}")
endif()
execute_process(COMMAND "${JAVA}" -cp "${OUTPUT_DIR}" org.litterbox.CrashStoreTest RESULT_VARIABLE tested)
if(NOT tested EQUAL 0)
    message(FATAL_ERROR "CrashStore behavioral oracle failed: ${tested}")
endif()
