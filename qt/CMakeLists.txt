# Litterbox Qt client shell: network API object, card store model,
# page-loading hook and fallback inbox view. Page content under qt/pages
# is owned by another slice; this shell only loads it when present.
cmake_minimum_required(VERSION 3.20)
project(litterbox-qt VERSION 0.1 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(Qt6 6.5 REQUIRED COMPONENTS Core Gui Qml Quick Network Sql Test QuickControls2)
find_package(KF6Kirigami CONFIG REQUIRED)

qt_standard_project_setup(REQUIRES 6.5)

enable_testing()

qt_add_executable(litterbox-qt
    src/main.cpp
    src/api.h
    src/api.cpp
    src/androidupdater.h
    src/androidupdater.cpp
    src/CardStore.h
    src/CardStore.cpp
    src/timerules.h
    src/timerules.cpp
)
target_compile_definitions(litterbox-qt PRIVATE LB_SOURCE_PAGES_DIR="${CMAKE_CURRENT_SOURCE_DIR}/pages")
set_property(TARGET litterbox-qt PROPERTY QT_ANDROID_PACKAGE_SOURCE_DIR "${CMAKE_CURRENT_SOURCE_DIR}/android")


set_source_files_properties(src/qml/CardActions.qml PROPERTIES QT_RESOURCE_ALIAS qml/CardActions.qml)

qt_add_qml_module(litterbox-qt
    URI litterbox
    VERSION 1.0
    QML_FILES
        qml/InboxView.qml
        src/qml/CardActions.qml
)

set(LB_KIRIGAMI_QML_IMPORT_PATH "" CACHE PATH "Kirigami QML import root for Android deployment")
if(LB_KIRIGAMI_QML_IMPORT_PATH)
    set_property(TARGET litterbox-qt PROPERTY QT_QML_IMPORT_PATH "${LB_KIRIGAMI_QML_IMPORT_PATH}")
endif()
target_link_libraries(litterbox-qt PRIVATE
    Qt6::Core
    Qt6::Gui
    Qt6::Qml
    Qt6::Quick
    Qt6::Network
    Qt6::Sql
    Qt6::QuickControls2
    KF6::Kirigami
)

# Reuse the canonical CardStore's isolated QtTest target.
add_subdirectory(tests)

qt_add_executable(tst_timerules tests/tst_timerules.cpp src/timerules.h src/timerules.cpp)
target_include_directories(tst_timerules PRIVATE src)
target_link_libraries(tst_timerules PRIVATE Qt6::Core Qt6::Test)
add_test(NAME timerules COMMAND tst_timerules)

qt_add_executable(tst_api tests/tst_api.cpp src/api.h src/api.cpp)
target_include_directories(tst_api PRIVATE src)
target_link_libraries(tst_api PRIVATE Qt6::Core Qt6::Qml Qt6::Network Qt6::Test)
add_test(NAME api COMMAND tst_api)
qt_add_executable(tst_androidupdater tests/tst_androidupdater.cpp src/androidupdater.h src/androidupdater.cpp)
target_include_directories(tst_androidupdater PRIVATE src)
target_link_libraries(tst_androidupdater PRIVATE Qt6::Core Qt6::Network Qt6::Test)
add_test(NAME androidupdater COMMAND tst_androidupdater)


# Page slice QMLTestUnit specs (owned by the screens slice), run headless.
file(GLOB _lb_page_tests "${CMAKE_CURRENT_SOURCE_DIR}/tests/tst_*.qml")
find_program(QMLTESTRUNNER6 NAMES qmltestrunner PATHS "/usr/lib/qt6/bin" NO_DEFAULT_PATH)
if (QMLTESTRUNNER6)
    foreach (_test ${_lb_page_tests})
        get_filename_component(_test_name ${_test} NAME_WE)
        add_test(NAME ${_test_name}
                 COMMAND ${QMLTESTRUNNER6} -input ${_test} -platform offscreen)
        set_tests_properties(${_test_name} PROPERTIES WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR})
    endforeach()
endif()

# Install desktop executables to bin and Android module libraries to lib.
install(TARGETS litterbox-qt RUNTIME DESTINATION bin LIBRARY DESTINATION lib)
install(FILES debian/litterbox-qt.desktop DESTINATION share/applications)
install(FILES debian/litterbox-qt.svg DESTINATION share/icons/hicolor/scalable/apps)
